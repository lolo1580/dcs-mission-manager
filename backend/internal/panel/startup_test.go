package panel

import "testing"

func TestStartupIsNotAnInput(t *testing.T) {
	for _, model := range []Model{PZ55, PZ70} {
		if got := Decode("device", model, nil, []byte{255, 255, 255}); len(got) != 0 {
			t.Fatalf("startup commands: %v", got)
		}
	}
	for _, control := range Controls(PZ70) {
		if control.ID == "AUTO_THROTTLE" && control.Kind != Toggle {
			t.Fatal("auto throttle must be a maintained switch")
		}
	}
}
