package smtpd

import (
	cryptoRand "crypto/rand"
	"encoding/base64"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

const _charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var charIndexes = len(_charset) - 1
var _counter = 0
var charmux sync.Mutex

// when crypto source is exhausted, fallback to PRNG, which must have a unique seed
func InitPseudoRandomNumberGeneratorFallback() {
	rand.Seed(time.Now().UnixNano())
}

func getCounter() string {
	charmux.Lock()
	_counter++
	if _counter > charIndexes {
		_counter = 0
	}
	charmux.Unlock()
	return string(_charset[_counter])
}

func randomInt(min, max int) int64 {
	return int64(rand.Intn(max-min) + min)
}

// NewMessageID generates a message ID, but make sure to seed the random number
// generator. It follows the Mailsac makeId pattern.
func NewMessageID() string {
	idLength := randomInt(13, 18)
	dateEntropy := strconv.FormatInt((time.Now().UnixNano()/int64(time.Millisecond))+idLength, 36)[4:]
	var randomPart []byte
	key := make([]byte, idLength)
	_, err := cryptoRand.Read(key[:])
	if err == nil {
		randomPart = key
	} else {
		// fallback to non-crypto random
		fallback := make([]byte, idLength)
		for i := range fallback {
			fallback[i] = _charset[rand.Intn(charIndexes)]
		}
		randomPart = fallback
	}
	randString := strings.Replace(base64.URLEncoding.EncodeToString(randomPart), "=", "", -1)
	// allow underscore as only special char, otherwise replace with a pseudo-rand char
	randString = strings.Replace(randString, "-", getCounter(), -1)
	randString = strings.Replace(randString, "/", getCounter(), -1)
	return dateEntropy + getCounter() + randString + getCounter()
}