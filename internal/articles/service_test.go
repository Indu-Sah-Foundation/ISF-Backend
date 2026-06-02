package articles

import "testing"

func TestFixNoTranslateSpacing(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "devanagari glued after span",
			in:   `<span class="notranslate">Janakpur</span>में`,
			want: `<span class="notranslate">Janakpur</span> में`,
		},
		{
			name: "devanagari glued before span",
			in:   `सहित<span class="notranslate">Neha</span>`,
			want: `सहित <span class="notranslate">Neha</span>`,
		},
		{
			name: "glued on both sides",
			in:   `यो<span class="notranslate">ISF</span>को`,
			want: `यो <span class="notranslate">ISF</span> को`,
		},
		{
			name: "already spaced — unchanged",
			in:   `यो <span class="notranslate">ISF</span> को`,
			want: `यो <span class="notranslate">ISF</span> को`,
		},
		{
			name: "punctuation after span — no space inserted",
			in:   `<span class="notranslate">ISF</span>, and more`,
			want: `<span class="notranslate">ISF</span>, and more`,
		},
		{
			name: "latin word glued (English target) — also fixed",
			in:   `the<span class="notranslate">ISF</span>team`,
			want: `the <span class="notranslate">ISF</span> team`,
		},
		{
			name: "no spans — unchanged",
			in:   `just some plain translated text.`,
			want: `just some plain translated text.`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := fixNoTranslateSpacing(c.in)
			if got != c.want {
				t.Fatalf("fixNoTranslateSpacing(%q)\n  = %q\n want %q", c.in, got, c.want)
			}
		})
	}
}
