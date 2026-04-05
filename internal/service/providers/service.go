package providers

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
	templatecategoriesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/template_categories"
	templatesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/templates"
)

type SmsSendingVehicle string

const (
	SmsSendingVehicleShortCode SmsSendingVehicle = "SHORT_CODE"
	SmsSendingVehicleLongCode  SmsSendingVehicle = "LONG_CODE"
)

type ProviderRepository interface {
	GetProviderDetailsByNotificationType(ctx context.Context, notificationType providersrepo.NotificationType, international bool) ([]providersrepo.ProviderDetail, error)
}

type TemplateRepository interface {
	GetTemplateByID(ctx context.Context, arg templatesrepo.GetTemplateByIDParams) (templatesrepo.Template, error)
}

type TemplateCategoryRepository interface {
	GetTemplateCategoryByID(ctx context.Context, id uuid.UUID) (templatecategoriesrepo.TemplateCategory, error)
}

type ClientResolver interface {
	GetClientByNameAndType(name string, notificationType providersrepo.NotificationType) (any, error)
}

type Service struct {
	cfg        *config.Config
	providers  ProviderRepository
	templates  TemplateRepository
	categories TemplateCategoryRepository
	resolver   ClientResolver
}

type Selection struct {
	Client         any
	Provider       providersrepo.ProviderDetail
	SendingVehicle SmsSendingVehicle
}

func NewService(cfg *config.Config, providers ProviderRepository, templates TemplateRepository, categories TemplateCategoryRepository, resolver ClientResolver) *Service {
	return &Service{cfg: cfg, providers: providers, templates: templates, categories: categories, resolver: resolver}
}

func (s *Service) ProviderToUse(ctx context.Context, notificationType providersrepo.NotificationType, sender, to string, templateID uuid.UUID, international bool) (Selection, error) {
	providers, err := s.providers.GetProviderDetailsByNotificationType(ctx, notificationType, international)
	if err != nil {
		return Selection{}, err
	}

	activeProviders := make([]providersrepo.ProviderDetail, 0, len(providers))
	for _, provider := range providers {
		if provider.Active {
			activeProviders = append(activeProviders, provider)
		}
	}

	if notificationType == providersrepo.NotificationTypeEmail {
		return s.selectProvider(activeProviders, notificationType, nil)
	}

	usingShortCodeTemplate, sendingVehicle, err := s.resolveSendingVehicle(ctx, templateID)
	if err != nil {
		return Selection{}, err
	}

	hasDedicatedNumber := strings.HasPrefix(strings.TrimSpace(sender), "+1")
	recipientRegion, regionKnown := classifyRecipientRegion(to)
	sendingToUSNumber := recipientRegion == "US"
	recipientOutsideCanada := regionKnown && recipientRegion != "CA" && recipientRegion != "US"
	cannotDetermineRecipientCountry := !regionKnown
	zone1OutsideCanada := recipientOutsideCanada && !international

	doNotUsePinpoint := (hasDedicatedNumber && !s.cfg.FFUsePinpointForDedicated) ||
		sendingToUSNumber ||
		cannotDetermineRecipientCountry ||
		zone1OutsideCanada ||
		strings.TrimSpace(s.cfg.AWSPinpointSCPoolID) == "" ||
		(strings.TrimSpace(s.cfg.AWSPinpointDefaultPoolID) == "" && !usingShortCodeTemplate)

	candidates := make([]providersrepo.ProviderDetail, 0, len(activeProviders))
	for _, provider := range activeProviders {
		if doNotUsePinpoint && provider.Identifier == "pinpoint" {
			continue
		}
		if !doNotUsePinpoint && provider.Identifier == "sns" {
			continue
		}
		candidates = append(candidates, provider)
	}

	return s.selectProvider(candidates, notificationType, &sendingVehicle)
}

func (s *Service) selectProvider(candidates []providersrepo.ProviderDetail, notificationType providersrepo.NotificationType, sendingVehicle *SmsSendingVehicle) (Selection, error) {
	if len(candidates) == 0 {
		return Selection{}, fmt.Errorf("no active %s providers", notificationType)
	}

	client, err := s.resolver.GetClientByNameAndType(candidates[0].Identifier, notificationType)
	if err != nil {
		return Selection{}, err
	}

	selection := Selection{Client: client, Provider: candidates[0], SendingVehicle: SmsSendingVehicleLongCode}
	if sendingVehicle != nil {
		selection.SendingVehicle = *sendingVehicle
	}
	return selection, nil
}

func (s *Service) resolveSendingVehicle(ctx context.Context, templateID uuid.UUID) (bool, SmsSendingVehicle, error) {
	if templateID == uuid.Nil || s.templates == nil || s.categories == nil {
		return false, SmsSendingVehicleLongCode, nil
	}

	template, err := s.templates.GetTemplateByID(ctx, templatesrepo.GetTemplateByIDParams{ID: templateID, ServiceID: uuid.MustParse(config.NotifyServiceID)})
	if err != nil {
		return false, SmsSendingVehicleLongCode, nil
	}
	if !template.TemplateCategoryID.Valid {
		return false, SmsSendingVehicleLongCode, nil
	}

	category, err := s.categories.GetTemplateCategoryByID(ctx, template.TemplateCategoryID.UUID)
	if err != nil {
		return false, SmsSendingVehicleLongCode, err
	}

	if category.SmsSendingVehicle == templatecategoriesrepo.SmsSendingVehicleShortCode {
		return true, SmsSendingVehicleShortCode, nil
	}

	return false, SmsSendingVehicleLongCode, nil
}

func classifyRecipientRegion(to string) (string, bool) {
	digits := normalizePhoneNumber(to)
	if len(digits) == 10 {
		digits = "1" + digits
	}
	if len(digits) != 11 || digits[0] != '1' {
		if strings.HasPrefix(strings.TrimSpace(to), "+") {
			return "INTL", true
		}
		return "", false
	}

	areaCode := digits[1:4]
	if areaCode == "671" {
		return "GU", true
	}
	if canadaAreaCodes[areaCode] {
		return "CA", true
	}
	return "US", true
}

func normalizePhoneNumber(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

var canadaAreaCodes = map[string]bool{
	"204": true, "226": true, "236": true, "249": true, "250": true, "263": true,
	"289": true, "306": true, "343": true, "354": true, "365": true, "367": true,
	"368": true, "382": true, "387": true, "403": true, "416": true, "418": true,
	"428": true, "431": true, "437": true, "438": true, "450": true, "468": true,
	"474": true, "506": true, "514": true, "519": true, "548": true, "579": true,
	"581": true, "584": true, "587": true, "600": true, "604": true, "613": true,
	"639": true, "647": true, "672": true, "683": true, "705": true, "709": true,
	"742": true, "753": true, "778": true, "780": true, "782": true, "807": true,
	"819": true, "825": true, "867": true, "873": true, "879": true, "902": true,
	"942": true,
}
