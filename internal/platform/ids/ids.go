package ids

import "crypto/rand"

func New(prefix string) string { return prefix + "_" + rand.Text() }
