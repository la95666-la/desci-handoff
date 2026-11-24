package strategies

import (
	"testing"

	"github.com/joincare/jc-release/internal/shared"
)

func TestCoreProductionStrategiesAreOperational(t *testing.T) {
	registry := New(t.TempDir())
	for _, component := range []shared.Component{shared.CompBackend, shared.CompFrontend, shared.CompAdmin} {
		s, err := registry.Get(component)
		if err != nil {
			t.Fatal(err)
		}
		if _, blocked := s.(*unavailableStrategy); blocked {
			t.Fatalf("%s is unexpectedly unavailable", component)
		}
	}
}

func TestAuxiliaryAndFinancialRuntimeStrategiesRemainUnavailable(t *testing.T) {
	registry := New(t.TempDir())
	for _, component := range []shared.Component{shared.CompTGBot, shared.CompLang} {
		s, err := registry.Get(component)
		if err != nil {
			t.Fatal(err)
		}
		if _, blocked := s.(*unavailableStrategy); !blocked {
			t.Fatalf("%s is unexpectedly operational", component)
		}
	}
}
