package emailutil

import (
	"strings"

	servicesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/services"
)

const AssetsBaseURL = "https://assets.notification.canada.ca/"

const (
	BrandTypeCustomLogo               = "custom_logo"
	BrandTypeBothEnglish              = "both_english"
	BrandTypeBothFrench               = "both_french"
	BrandTypeLogoWithBackgroundColour = "custom_logo_with_background_colour"
	BrandTypeNoBranding               = "no_branding"
)

type HTMLEmailOptions struct {
	FIPBannerEnglish         bool
	FIPBannerFrench          bool
	LogoWithBackgroundColour bool
	BrandColour              *string
	BrandText                *string
	BrandName                *string
	BrandLogo                *string
}

func GetHTMLEmailOptions(branding *servicesrepo.EmailBranding) HTMLEmailOptions {
	if branding == nil || strings.TrimSpace(branding.BrandType) == "" || branding.BrandType == BrandTypeNoBranding {
		return HTMLEmailOptions{FIPBannerEnglish: true}
	}

	options := HTMLEmailOptions{
		BrandName: ptr(strings.TrimSpace(branding.Name)),
	}

	switch branding.BrandType {
	case BrandTypeCustomLogo:
	case BrandTypeBothEnglish:
		options.FIPBannerEnglish = true
	case BrandTypeBothFrench:
		options.FIPBannerFrench = true
	case BrandTypeLogoWithBackgroundColour:
		options.LogoWithBackgroundColour = true
	default:
		options.FIPBannerEnglish = true
	}

	if branding.Colour.Valid {
		options.BrandColour = ptr(strings.TrimSpace(branding.Colour.String))
	}
	if branding.Text.Valid {
		options.BrandText = ptr(strings.TrimSpace(branding.Text.String))
	}
	if branding.Logo.Valid {
		logo := strings.TrimSpace(branding.Logo.String)
		if logo != "" {
			options.BrandLogo = ptr(AssetsBaseURL + logo)
		}
	}

	return options
}

func ptr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
