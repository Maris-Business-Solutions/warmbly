package msgraph

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestWarmupFilingDoesNotMoveAlreadyFiledGraphMessage(t *testing.T) {
	for _, archive := range []bool{false, true} {
		t.Run(map[bool]string{false: "folder", true: "archive"}[archive], func(t *testing.T) {
			moves := 0
			parent := "inbox-id"
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/move"):
					moves++
					parent = "target-id"
					fmt.Fprint(w, `{"id":"new-id"}`)
				case strings.HasSuffix(r.URL.Path, "/mailFolders/archive"):
					fmt.Fprint(w, `{"id":"target-id"}`)
				case strings.HasSuffix(r.URL.Path, "/mailFolders"):
					fmt.Fprint(w, `{"value":[{"id":"target-id","displayName":"Warmbly"}]}`)
				default:
					fmt.Fprintf(w, `{"parentFolderId":%q}`, parent)
				}
			}))
			defer srv.Close()
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: rewriteTo(srv.URL)})
			client := &Client{}
			if err := client.Init(ctx, &oauth2.Token{AccessToken: "test", Expiry: time.Now().Add(time.Hour)}, oauth2.Config{}); err != nil {
				t.Fatal(err)
			}
			move := func(id string) (string, error) {
				if archive {
					return client.MoveToArchive(ctx, id)
				}
				return client.MoveToFolder(ctx, id, "Warmbly")
			}
			id, err := move("old-id")
			if err != nil || id != "new-id" || moves != 1 {
				t.Fatalf("initial filing: id=%q moves=%d err=%v", id, moves, err)
			}
			id, err = move("new-id")
			if err != nil || id != "new-id" || moves != 1 {
				t.Fatalf("repeated filing moved message again: id=%q moves=%d err=%v", id, moves, err)
			}
		})
	}
}
