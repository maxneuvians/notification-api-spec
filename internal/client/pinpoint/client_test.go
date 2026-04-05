package pinpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	clientpkg "github.com/maxneuvians/notification-api-spec/internal/client"
	"github.com/maxneuvians/notification-api-spec/internal/config"
)

type stubSenderAPI struct {
	input    SendTextMessageInput
	response string
	err      error
	called   bool
}

func (s *stubSenderAPI) SendTextMessage(_ context.Context, input SendTextMessageInput) (string, error) {
	s.called = true
	s.input = input
	return s.response, s.err
}

func TestSendSMSNormalisationAndPools(t *testing.T) {
	templateID := uuid.New()
	serviceID, _ := uuid.Parse(config.NotifyServiceID)

	tests := []struct {
		name               string
		cfg                config.Config
		request            clientpkg.SMSRequest
		wantDestination    string
		wantOrigination    string
		wantHasOrigination bool
		wantHasDryRun      bool
		useDedicated       bool
	}{
		{
			name:               "10 digit adds +1 and default pool",
			cfg:                config.Config{AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg"},
			request:            clientpkg.SMSRequest{To: "6135550100", Content: "hello"},
			wantDestination:    "+16135550100",
			wantOrigination:    "default",
			wantHasOrigination: true,
		},
		{
			name:               "short code vehicle uses sc pool",
			cfg:                config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg"},
			request:            clientpkg.SMSRequest{To: "+16135550100", Content: "hello", SendingVehicle: "short_code"},
			wantDestination:    "+16135550100",
			wantOrigination:    "sc",
			wantHasOrigination: true,
		},
		{
			name:               "sc template for notify service uses sc pool",
			cfg:                config.Config{AWSPinpointSCPoolID: "sc", AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg", AWSPinpointSCTemplateIDs: []string{templateID.String()}},
			request:            clientpkg.SMSRequest{To: "+16135550100", Content: "hello", TemplateID: templateID, ServiceID: serviceID},
			wantDestination:    "+16135550100",
			wantOrigination:    "sc",
			wantHasOrigination: true,
		},
		{
			name:            "international omits origination and dry run",
			cfg:             config.Config{AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg"},
			request:         clientpkg.SMSRequest{To: "+442071234567", Content: "hello"},
			wantDestination: "+442071234567",
		},
		{
			name:               "external test number enables dry run",
			cfg:                config.Config{AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg"},
			request:            clientpkg.SMSRequest{To: config.ExternalTestNumber, Content: "hello"},
			wantDestination:    config.ExternalTestNumber,
			wantOrigination:    "default",
			wantHasOrigination: true,
			wantHasDryRun:      true,
		},
		{
			name:         "dedicated sender uses dedicated client",
			cfg:          config.Config{AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg", FFUsePinpointForDedicated: true},
			request:      clientpkg.SMSRequest{To: "+16135550100", Content: "hello", Sender: "+16135559999"},
			wantDestination:    "+16135550100",
			wantOrigination:    "+16135559999",
			wantHasOrigination: true,
			useDedicated: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defaultSender := &stubSenderAPI{response: "ok"}
			dedicatedSender := &stubSenderAPI{response: "ok"}
			pinpointClient := NewClient(&tc.cfg, defaultSender, dedicatedSender)

			if _, err := pinpointClient.SendSMS(context.Background(), tc.request); err != nil {
				t.Fatalf("SendSMS() error = %v", err)
			}

			used := defaultSender
			if tc.useDedicated {
				used = dedicatedSender
				if defaultSender.called {
					t.Fatal("default client should not be called for dedicated sender")
				}
			}

			if used.input.DestinationPhoneNumber != tc.wantDestination {
				t.Fatalf("destination = %q, want %q", used.input.DestinationPhoneNumber, tc.wantDestination)
			}
			if used.input.MessageType != transactionalMessageType {
				t.Fatalf("message type = %q, want %q", used.input.MessageType, transactionalMessageType)
			}
			if used.input.ConfigurationSetName != tc.cfg.AWSPinpointConfigSet {
				t.Fatalf("config set = %q, want %q", used.input.ConfigurationSetName, tc.cfg.AWSPinpointConfigSet)
			}
			if used.input.HasOriginationIdentity != tc.wantHasOrigination {
				t.Fatalf("has origination = %v, want %v", used.input.HasOriginationIdentity, tc.wantHasOrigination)
			}
			if used.input.OriginationIdentity != tc.wantOrigination {
				t.Fatalf("origination = %q, want %q", used.input.OriginationIdentity, tc.wantOrigination)
			}
			if used.input.HasDryRun != tc.wantHasDryRun {
				t.Fatalf("has dry run = %v, want %v", used.input.HasDryRun, tc.wantHasDryRun)
			}
		})
	}
}

func TestSendSMSValidationAndConflictMapping(t *testing.T) {
	t.Run("empty recipient returns error", func(t *testing.T) {
		pinpointClient := NewClient(&config.Config{}, &stubSenderAPI{}, &stubSenderAPI{})
		if _, err := pinpointClient.SendSMS(context.Background(), clientpkg.SMSRequest{To: ""}); err == nil || err.Error() != "No valid numbers found for SMS delivery" {
			t.Fatalf("SendSMS() error = %v", err)
		}
	})

	t.Run("opted out returns string", func(t *testing.T) {
		defaultSender := &stubSenderAPI{err: &APIError{Code: "ConflictException", Reason: "DESTINATION_PHONE_NUMBER_OPTED_OUT", Message: "opted out"}}
		pinpointClient := NewClient(&config.Config{AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg"}, defaultSender, nil)
		got, err := pinpointClient.SendSMS(context.Background(), clientpkg.SMSRequest{To: "+16135550100", Content: "hello"})
		if err != nil {
			t.Fatalf("SendSMS() error = %v", err)
		}
		if got != "opted_out" {
			t.Fatalf("response = %q, want opted_out", got)
		}
	})

	t.Run("other conflict raises PinpointConflictException", func(t *testing.T) {
		defaultSender := &stubSenderAPI{err: &APIError{Code: "ConflictException", Reason: "OTHER", Message: "conflict"}}
		pinpointClient := NewClient(&config.Config{AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg"}, defaultSender, nil)
		_, err := pinpointClient.SendSMS(context.Background(), clientpkg.SMSRequest{To: "+16135550100", Content: "hello"})
		var target *PinpointConflictException
		if !errors.As(err, &target) || target.Reason != "OTHER" {
			t.Fatalf("error = %v, want PinpointConflictException with OTHER reason", err)
		}
	})

	t.Run("validation raises PinpointValidationException", func(t *testing.T) {
		defaultSender := &stubSenderAPI{err: &APIError{Code: "ValidationException", Reason: "NO_ORIGINATION_IDENTITIES_FOUND", Message: "bad"}}
		pinpointClient := NewClient(&config.Config{AWSPinpointDefaultPoolID: "default", AWSPinpointConfigSet: "cfg"}, defaultSender, nil)
		_, err := pinpointClient.SendSMS(context.Background(), clientpkg.SMSRequest{To: "+16135550100", Content: "hello"})
		var target *PinpointValidationException
		if !errors.As(err, &target) || target.Reason != "NO_ORIGINATION_IDENTITIES_FOUND" {
			t.Fatalf("error = %v, want PinpointValidationException with preserved reason", err)
		}
	})
}