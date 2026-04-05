package emailutil

import (
	"database/sql"
	"testing"

	"github.com/google/uuid"

	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

func TestGetHTMLEmailOptionsBrandingMatrix(t *testing.T) {
	tests := []struct {
		name           string
		branding       *servicesrepo.EmailBranding
		wantEnglish    bool
		wantFrench     bool
		wantBackground bool
	}{
		{name: "nil branding uses default banner", branding: nil, wantEnglish: true},
		{name: "custom logo", branding: branding(BrandTypeCustomLogo, "logo.png"), wantEnglish: false, wantFrench: false, wantBackground: false},
		{name: "both english", branding: branding(BrandTypeBothEnglish, "logo.png"), wantEnglish: true},
		{name: "both french", branding: branding(BrandTypeBothFrench, "logo.png"), wantFrench: true},
		{name: "background logo", branding: branding(BrandTypeLogoWithBackgroundColour, "logo.png"), wantBackground: true},
		{name: "no branding", branding: branding(BrandTypeNoBranding, "logo.png"), wantEnglish: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetHTMLEmailOptions(tc.branding)
			if got.FIPBannerEnglish != tc.wantEnglish || got.FIPBannerFrench != tc.wantFrench || got.LogoWithBackgroundColour != tc.wantBackground {
				t.Fatalf("options = %#v", got)
			}
		})
	}
}

func TestGetHTMLEmailOptionsPrefixesBrandLogo(t *testing.T) {
	got := GetHTMLEmailOptions(branding(BrandTypeCustomLogo, "org/logo.png"))
	if got.BrandLogo == nil || *got.BrandLogo != AssetsBaseURL+"org/logo.png" {
		t.Fatalf("brand logo = %#v, want prefixed asset url", got.BrandLogo)
	}
}

func TestGetHTMLEmailOptionsAllowsNilLogoForBackgroundBranding(t *testing.T) {
	item := branding(BrandTypeLogoWithBackgroundColour, "")
	item.Logo = sql.NullString{}
	got := GetHTMLEmailOptions(item)
	if got.BrandLogo != nil {
		t.Fatalf("brand logo = %#v, want nil", got.BrandLogo)
	}
}

func branding(brandType, logo string) *servicesrepo.EmailBranding {
	return &servicesrepo.EmailBranding{
		ID:        uuid.New(),
		Name:      "Brand",
		BrandType: brandType,
		Colour:    sql.NullString{String: "#123456", Valid: true},
		Text:      sql.NullString{String: "Text", Valid: true},
		Logo:      sql.NullString{String: logo, Valid: logo != ""},
	}
}
