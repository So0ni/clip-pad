package utils

import (
"strings"
"testing"
)

func TestGenerateIDLengthAndCharset(t *testing.T) {
id, err := GenerateID(8)
if err != nil {
t.Fatalf("GenerateID() error = %v", err)
}
if len(id) != 8 {
t.Fatalf("len(id) = %d, want 8", len(id))
}
for _, char := range id {
if !strings.ContainsRune(Charset, char) {
t.Fatalf("unexpected character %q", char)
}
}
}

func TestGenerateIDLowCollision(t *testing.T) {
seen := make(map[string]struct{})
for i := 0; i < 2000; i++ {
id, err := GenerateID(8)
if err != nil {
t.Fatalf("GenerateID() error = %v", err)
}
if _, exists := seen[id]; exists {
t.Fatalf("duplicate id generated: %s", id)
}
seen[id] = struct{}{}
}
}
