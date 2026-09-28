package mailboxavatar

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/infrastructure/storage"
	"github.com/warmbly/warmbly/internal/repository"
)

type fakeRepo struct {
	url     map[uuid.UUID]string
	checked map[uuid.UUID]int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{url: map[uuid.UUID]string{}, checked: map[uuid.UUID]int{}}
}

func (r *fakeRepo) Due(context.Context, time.Time, int) ([]repository.MailboxAvatarCandidate, error) {
	return nil, nil
}
func (r *fakeRepo) Current(_ context.Context, _, id uuid.UUID) (string, error) { return r.url[id], nil }
func (r *fakeRepo) Set(_ context.Context, _, id uuid.UUID, u string) error {
	r.url[id] = u
	r.checked[id]++
	return nil
}
func (r *fakeRepo) Checked(_ context.Context, _, id uuid.UUID) error {
	r.checked[id]++
	return nil
}

func newStore(t *testing.T) storage.Store {
	t.Helper()
	st, err := storage.NewFilesystem(t.TempDir(), "http://localhost/public")
	if err != nil {
		t.Fatal(err)
	}
	return st
}

type fakeGrant struct {
	data []byte
	err  error
}

func (g fakeGrant) MailboxPhoto(context.Context, uuid.UUID) ([]byte, error) { return g.data, g.err }

func pngPhoto(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestSaveIsContentAddressed(t *testing.T) {
	ctx := context.Background()
	repo, st := newFakeRepo(), newStore(t)
	s := New(repo, st, nil, nil)
	org, id := uuid.New(), uuid.New()

	if err := s.Save(ctx, org, id, pngPhoto(t, color.White)); err != nil {
		t.Fatal(err)
	}
	first := repo.url[id]
	if !strings.Contains(first, KeyPrefix+id.String()+"-") || !strings.HasSuffix(first, ".jpg") {
		t.Fatalf("stored url = %q", first)
	}
	// The same photo again keeps the object and only records the check.
	if err := s.Save(ctx, org, id, pngPhoto(t, color.White)); err != nil {
		t.Fatal(err)
	}
	if repo.url[id] != first {
		t.Fatalf("an unchanged photo was stored again: %q", repo.url[id])
	}
	// A new photo replaces the old object.
	if err := s.Save(ctx, org, id, pngPhoto(t, color.Black)); err != nil {
		t.Fatal(err)
	}
	if repo.url[id] == first {
		t.Fatal("a changed photo kept the old url")
	}
	if ok, _ := st.Has(ctx, keyOf(first)); ok {
		t.Fatal("the replaced photo's object was left behind")
	}
}

func TestSaveRefusesWhatIsNotAPhoto(t *testing.T) {
	s := New(newFakeRepo(), newStore(t), nil, nil)
	for _, data := range [][]byte{nil, []byte("<svg onload=alert(1)>"), bytes.Repeat([]byte{0xff}, maxBytes+1)} {
		if err := s.Save(context.Background(), uuid.New(), uuid.New(), data); err != ErrNotImage {
			t.Fatalf("Save(%d bytes) = %v, want ErrNotImage", len(data), err)
		}
	}
}

func TestRefreshClearsAPhotoTheSourceDropped(t *testing.T) {
	ctx := context.Background()
	repo, st := newFakeRepo(), newStore(t)
	org, id, grant := uuid.New(), uuid.New(), uuid.New()
	if err := New(repo, st, nil, nil).Save(ctx, org, id, pngPhoto(t, color.White)); err != nil {
		t.Fatal(err)
	}
	c := repository.MailboxAvatarCandidate{ID: id, OrganizationID: org, AvatarURL: repo.url[id], DomainGrantID: &grant}
	New(repo, st, fakeGrant{}, nil).refresh(ctx, c, map[uuid.UUID]map[string]string{})
	if repo.url[id] != "" {
		t.Fatalf("a photo the grant no longer has is still shown: %q", repo.url[id])
	}
}

func TestDownloadRefusesNonHTTPS(t *testing.T) {
	s := New(newFakeRepo(), newStore(t), nil, nil)
	for _, u := range []string{"http://example.com/a.jpg", "file:///etc/passwd", "ftp://x/y", "data:text/html;base64,PGI+"} {
		if _, err := s.download(context.Background(), u); err != ErrNotImage {
			t.Fatalf("download(%q) = %v, want ErrNotImage", u, err)
		}
	}
	photo := pngPhoto(t, color.White)
	got, err := s.download(context.Background(), "data:image/png;base64,"+base64.StdEncoding.EncodeToString(photo))
	if err != nil || !bytes.Equal(got, photo) {
		t.Fatalf("data url = %d bytes, %v", len(got), err)
	}
}

func TestRefreshKeepsThePhotoWhenTheGrantCannotBeAsked(t *testing.T) {
	ctx := context.Background()
	repo, st := newFakeRepo(), newStore(t)
	org, id, grant := uuid.New(), uuid.New(), uuid.New()
	if err := New(repo, st, nil, nil).Save(ctx, org, id, pngPhoto(t, color.White)); err != nil {
		t.Fatal(err)
	}
	kept := repo.url[id]
	c := repository.MailboxAvatarCandidate{ID: id, OrganizationID: org, AvatarURL: kept, DomainGrantID: &grant}
	New(repo, st, fakeGrant{err: errors.New("grant inactive")}, nil).refresh(ctx, c, map[uuid.UUID]map[string]string{})
	if repo.url[id] != kept {
		t.Fatalf("an unanswerable grant cleared the photo: %q", repo.url[id])
	}
}

type fakeVendor struct{ pics map[string]string }

func (v fakeVendor) Pictures(context.Context, uuid.UUID, uuid.UUID) (map[string]string, error) {
	return v.pics, nil
}

// A vendor with no picture does not turn an unanswered grant into "no photo".
func TestRefreshKeepsThePhotoWhenTheGrantFailsAndTheVendorHasNone(t *testing.T) {
	ctx := context.Background()
	repo, st := newFakeRepo(), newStore(t)
	org, id, grant, conn := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := New(repo, st, nil, nil).Save(ctx, org, id, pngPhoto(t, color.White)); err != nil {
		t.Fatal(err)
	}
	kept := repo.url[id]
	c := repository.MailboxAvatarCandidate{ID: id, OrganizationID: org, AvatarURL: kept, DomainGrantID: &grant, VendorConnectionID: &conn, VendorMailboxID: "mb-1"}
	New(repo, st, fakeGrant{err: errors.New("grant inactive")}, fakeVendor{pics: map[string]string{}}).refresh(ctx, c, map[uuid.UUID]map[string]string{})
	if repo.url[id] != kept {
		t.Fatalf("an unanswered grant with an empty vendor cleared the photo: %q", repo.url[id])
	}
}
