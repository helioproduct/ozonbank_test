package pagination

import (
	"encoding/base64"
	"encoding/json"
	"time"
)

type Cursor struct {
	CreatedAt time.Time
	ID        int64
}

func (c *Cursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.StdEncoding.EncodeToString(b)
}

func Decode(s *string) (*Cursor, error) {
	if s == nil || *s == "" {
		return nil, nil
	}

	data, err := base64.StdEncoding.DecodeString(*s)
	if err != nil {
		return nil, err
	}
	var c Cursor
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
