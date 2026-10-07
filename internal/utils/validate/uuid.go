package validate

import (
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
)

func Uuid(id string) (*string, *errx.Error) {
	if id == "" {
		return nil, nil
	}

	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errx.ErrUuid
	}

	uidstr := uid.String()

	return &uidstr, nil
}

func Uuids(ids []string) ([]string, *errx.Error) {
	if ids == nil {
		return nil, nil
	}
	out := make([]string, 0, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil {
			return nil, errx.ErrUuid
		}
		if _, ok := seen[parsed]; !ok {
			out = append(out, parsed.String())
			seen[parsed] = struct{}{}
		}
	}
	return out, nil
}
