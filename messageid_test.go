package smtpd_test

import (
	"fmt"
	"github.com/ruffrey/smtpd"
	"testing"
)

func Test_MessageID(t *testing.T) {
	t.Run("NewMessageID is unlikely to collide", func(t *testing.T) {
		smtpd.InitPseudoRandomNumberGeneratorFallback()
		o := make(map[string]bool)
		var id string
		for i := 0; i < 1000000; i++ {
			id = smtpd.NewMessageID()
			if i % 500000 == 0 {
				fmt.Println("NewMessageID test: ", id)
			}
			if o[id] {
				t.Errorf("Got duplicate unique id %d (%s)", i, id)
				t.FailNow()
			}
			o[id] = true
		}
	})
}
