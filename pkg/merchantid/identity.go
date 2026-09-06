// Package merchantid handles the legacy spelling of a single merchant identity
// at contract boundaries. It neither creates IDs nor authorizes access.
package merchantid

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var ErrConflict = errors.New("conflicting merchant identity")

// Resolve accepts either spelling, never two different identities. Blank values
// remain blank so each use case can enforce its own required-context rules.
func Resolve(merchant, legacy string) (string, error) {
	if merchant != "" && legacy != "" && merchant != legacy {
		return "", ErrConflict
	}
	if merchant != "" {
		return merchant, nil
	}
	return legacy, nil
}

// Normalize fills both spellings in decoded RPC messages for v1 compatibility.
// Call before business logic and when adapting responses. Existing immutable
// domain snapshots, database columns and idempotency payloads are not rewritten.
func Normalize(value any) error {
	m, ok := value.(proto.Message)
	if !ok || m == nil {
		return nil
	}
	return normalize(m.ProtoReflect())
}

func normalize(m protoreflect.Message) error {
	if !m.IsValid() {
		return nil
	}
	fields := m.Descriptor().Fields()
	canonical, legacy := fields.ByName("merchant_id"), fields.ByName("tenant_id")
	if canonical != nil && legacy != nil {
		id, err := Resolve(m.Get(canonical).String(), m.Get(legacy).String())
		if err != nil {
			return err
		}
		if id != "" {
			m.Set(canonical, protoreflect.ValueOfString(id))
			m.Set(legacy, protoreflect.ValueOfString(id))
		}
	}
	var err error
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.Kind() != protoreflect.MessageKind || f.IsMap() {
			return true
		}
		if f.IsList() {
			list := v.List()
			for i := 0; i < list.Len() && err == nil; i++ {
				err = normalize(list.Get(i).Message())
			}
		} else {
			err = normalize(v.Message())
		}
		return err == nil
	})
	return err
}
