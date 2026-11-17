package identity

import "testing"

func TestNormalizeLoginEmailAndImmutableHistoricalBinding(t *testing.T) {
	for input, want := range map[string]string{" A@EXAMPLE.COM ": "a@example.com", "": "", "not-email": "", "Name <a@example.com>": ""} {
		if got := NormalizeLoginEmail(input); got != want {
			t.Fatalf("normalize %q=%q want=%q", input, got, want)
		}
	}
	for _, email := range []string{"", " ", " A@example.com ", "historical-unvalidated"} {
		binding := HistoricalEmailBinding(email)
		if binding.Available || binding.Verified != (binding.Email != "") {
			t.Fatalf("binding=%+v", binding)
		}
	}
}
