package providers

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/maxneuvians/notification-api-spec/internal/config"
	providersrepo "github.com/maxneuvians/notification-api-spec/internal/repository/providers"
	templatecategoriesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/template_categories"
	templatesrepo "github.com/maxneuvians/notification-api-spec/internal/repository/templates"
)

type stubProviderRepo struct {
	providers []providersrepo.ProviderDetail
	err       error
}

func (s *stubProviderRepo) GetProviderDetailsByNotificationType(_ context.Context, _ providersrepo.NotificationType, _ bool) ([]providersrepo.ProviderDetail, error) {
	return s.providers, s.err
}

type stubTemplateRepo struct {
	template templatesrepo.Template
	err      error
}

func (s *stubTemplateRepo) GetTemplateByID(_ context.Context, _ templatesrepo.GetTemplateByIDParams) (templatesrepo.Template, error) {
	if s.err != nil {
		return templatesrepo.Template{}, s.err
	}
	return s.template, nil
}

type stubCategoryRepo struct {
	category templatecategoriesrepo.TemplateCategory
	err      error
}

func (s *stubCategoryRepo) GetTemplateCategoryByID(_ context.Context, _ uuid.UUID) (templatecategoriesrepo.TemplateCategory, error) {
	if s.err != nil {
		return templatecategoriesrepo.TemplateCategory{}, s.err
	}
	return s.category, nil
}

type stubResolver struct {
	name string
	err  error
}

func (s *stubResolver) GetClientByNameAndType(name string, _ providersrepo.NotificationType) (any, error) {
	s.name = name
	if s.err != nil {
		return nil, s.err
	}
	return name, nil
}

func TestProviderToUseSMSRouting(t *testing.T) {
	templateCategoryID := uuid.New()
	templateID := uuid.New()
	baseProviders := []providersrepo.ProviderDetail{
		{ID: uuid.New(), Identifier: "sns", DisplayName: "SNS", NotificationType: providersrepo.NotificationTypeSms, Priority: 10, Active: true},
		{ID: uuid.New(), Identifier: "pinpoint", DisplayName: "Pinpoint", NotificationType: providersrepo.NotificationTypeSms, Priority: 20, Active: true},
	}

	tests := []struct {
		name          string
		cfg           config.Config
		sender        string
		to            string
		templateID    uuid.UUID
		international bool
		template      templatesrepo.Template
		category      templatecategoriesrepo.TemplateCategory
		providers     []providersrepo.ProviderDetail
		wantProvider  string
		wantVehicle   SmsSendingVehicle
		wantErr       string
	}{
		{
			name:         "US number routes to SNS",
			cfg:          config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default"},
			to:           "+17065551234",
			providers:    baseProviders,
			wantProvider: "sns",
			wantVehicle:  SmsSendingVehicleLongCode,
		},
		{
			name:         "Canadian number routes to Pinpoint",
			cfg:          config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default"},
			to:           "+16135551234",
			providers:    baseProviders,
			wantProvider: "pinpoint",
			wantVehicle:  SmsSendingVehicleLongCode,
		},
		{
			name:         "Dedicated sender with flag off routes to SNS",
			cfg:          config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default", FFUsePinpointForDedicated: false},
			sender:       "+16135551234",
			to:           "+16135550000",
			providers:    baseProviders,
			wantProvider: "sns",
			wantVehicle:  SmsSendingVehicleLongCode,
		},
		{
			name:         "SC template without default pool routes to Pinpoint",
			cfg:          config.Config{AWSPinpointSCPoolID: "sc"},
			to:           "+16135551234",
			templateID:   templateID,
			template:     templatesrepo.Template{ID: templateID, TemplateCategoryID: uuid.NullUUID{UUID: templateCategoryID, Valid: true}},
			category:     templatecategoriesrepo.TemplateCategory{ID: templateCategoryID, SmsSendingVehicle: templatecategoriesrepo.SmsSendingVehicleShortCode},
			providers:    baseProviders,
			wantProvider: "pinpoint",
			wantVehicle:  SmsSendingVehicleShortCode,
		},
		{
			name:         "zone 1 outside Canada routes to SNS",
			cfg:          config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default"},
			to:           "+16715551234",
			providers:    baseProviders,
			wantProvider: "sns",
			wantVehicle:  SmsSendingVehicleLongCode,
		},
		{
			name:         "unparseable number routes to SNS",
			cfg:          config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default"},
			to:           "not-a-phone-number",
			providers:    baseProviders,
			wantProvider: "sns",
			wantVehicle:  SmsSendingVehicleLongCode,
		},
		{
			name:      "all inactive returns error",
			cfg:       config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default"},
			to:        "+16135551234",
			providers: []providersrepo.ProviderDetail{{Identifier: "sns", NotificationType: providersrepo.NotificationTypeSms, Priority: 10}, {Identifier: "pinpoint", NotificationType: providersrepo.NotificationTypeSms, Priority: 20}},
			wantErr:   "no active sms providers",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &stubResolver{}
			svc := NewService(&tc.cfg, &stubProviderRepo{providers: tc.providers}, &stubTemplateRepo{template: tc.template, err: errors.New("")}, &stubCategoryRepo{category: tc.category}, resolver)
			if tc.templateID != uuid.Nil {
				svc.templates = &stubTemplateRepo{template: tc.template}
			}
			if tc.category.ID != uuid.Nil {
				svc.categories = &stubCategoryRepo{category: tc.category}
			}

			selection, err := svc.ProviderToUse(context.Background(), providersrepo.NotificationTypeSms, tc.sender, tc.to, tc.templateID, tc.international)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("ProviderToUse() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProviderToUse() error = %v", err)
			}
			if selection.Provider.Identifier != tc.wantProvider {
				t.Fatalf("provider = %q, want %q", selection.Provider.Identifier, tc.wantProvider)
			}
			if selection.SendingVehicle != tc.wantVehicle {
				t.Fatalf("sending vehicle = %q, want %q", selection.SendingVehicle, tc.wantVehicle)
			}
		})
	}
}

func TestProviderToUseEmailAlwaysUsesFirstActiveProvider(t *testing.T) {
	resolver := &stubResolver{}
	svc := NewService(&config.Config{}, &stubProviderRepo{providers: []providersrepo.ProviderDetail{{Identifier: "ses", NotificationType: providersrepo.NotificationTypeEmail, Priority: 5, Active: true}}}, nil, nil, resolver)

	selection, err := svc.ProviderToUse(context.Background(), providersrepo.NotificationTypeEmail, "", "", uuid.Nil, false)
	if err != nil {
		t.Fatalf("ProviderToUse() error = %v", err)
	}
	if selection.Provider.Identifier != "ses" {
		t.Fatalf("provider = %q, want ses", selection.Provider.Identifier)
	}
}
