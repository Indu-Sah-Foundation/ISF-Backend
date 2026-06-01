package maintenance

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name        string
		title       string
		description string
		want        Target
	}{
		{
			name:  "frontend ui",
			title: "The donate button is cut off on iPad",
			want:  TargetFrontend,
		},
		{
			name:        "frontend styling",
			title:       "Navbar color is wrong on mobile",
			description: "The header font and layout look broken on small screens.",
			want:        TargetFrontend,
		},
		{
			name:        "backend api",
			title:       "API returns 500 on the donations endpoint",
			description: "The postgres database query panics in the payments handler.",
			want:        TargetBackend,
		},
		{
			name:  "backend auth",
			title: "Admin login JWT token is rejected by the server",
			want:  TargetBackend,
		},
		{
			name:        "infra azure",
			title:       "DNS not resolving after the Azure deploy",
			description: "Terraform changed the domain and the SSL certificate is failing.",
			want:        TargetInfra,
		},
		{
			name:  "infra pipeline",
			title: "GitHub Actions deployment workflow is failing on the container registry",
			want:  TargetInfra,
		},
		{
			name:  "ambiguous falls back to frontend",
			title: "Something is wrong",
			want:  TargetFrontend,
		},
		{
			name:  "empty falls back to frontend",
			title: "",
			want:  TargetFrontend,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(c.title, c.description)
			if got != c.want {
				t.Fatalf("Classify(%q,%q) = %q, want %q", c.title, c.description, got, c.want)
			}
		})
	}
}
