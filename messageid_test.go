package smtpd_test

import (
	"github.com/ruffrey/smtpd"
	"math/rand"
	"testing"
	"time"
)

func Test_MessageID(t *testing.T) {
	t.Run("NewMessageID is unlikely to collide", func(t *testing.T) {
		rand.Seed(time.Now().UnixNano())
		o := make(map[string]bool)
		var id string
		for i := 0; i < 1000000; i++ {
			id = smtpd.NewMessageID()
			if o[id] {
				t.Errorf("Got duplicate unique id %d (%s)", i, id)
				t.FailNow()
			}
			o[id] = true
		}
	})
}
