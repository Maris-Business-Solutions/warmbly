package email

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func TestUpdateRejectsMalformedTagsBeforeRepository(t *testing.T) {
	for _, tags := range [][]string{{"cro-c6"}, {uuid.NewString(), "cro-c6"}, {""}} {
		_, err := (&emailService{}).Update(context.Background(), "org", "user", "email", &models.UpdateEmail{Tags: tags})
		if err == nil || err.Code != errx.BadRequest {
			t.Fatalf("error=%v; want bad_request", err)
		}
	}
}
