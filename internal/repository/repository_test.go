package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

type fakeDB struct {
	query string
	err   error
}

func (f *fakeDB) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	f.query = placeholder
	return nil, f.err
}

func (f *fakeDB) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, nil
}

func (f *fakeDB) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (f *fakeDB) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

type enumCase struct {
	name      string
	scan      func(any) (string, error)
	valid     func(string) bool
	all       func() []string
	nullScan  func(any) (string, bool, error)
	nullValue func(string, bool) (driver.Value, error)
}

func TestEnumHelpers(t *testing.T) {
	tests := []enumCase{
		{
			name: "InvitedUsersStatusTypes",
			scan: func(src any) (string, error) {
				var v InvitedUsersStatusTypes
				err := v.Scan(src)
				return string(v), err
			},
			valid: func(v string) bool { return InvitedUsersStatusTypes(v).Valid() },
			all:   func() []string { return invitedUsersStatusTypesValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullInvitedUsersStatusTypes
				err := v.Scan(src)
				return string(v.InvitedUsersStatusTypes), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullInvitedUsersStatusTypes{InvitedUsersStatusTypes: InvitedUsersStatusTypes(v), Valid: valid}.Value()
			},
		},
		{
			name:  "JobStatusTypes",
			scan:  func(src any) (string, error) { var v JobStatusTypes; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return JobStatusTypes(v).Valid() },
			all:   func() []string { return jobStatusTypesValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullJobStatusTypes
				err := v.Scan(src)
				return string(v.JobStatusTypes), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullJobStatusTypes{JobStatusTypes: JobStatusTypes(v), Valid: valid}.Value()
			},
		},
		{
			name: "NotificationFeedbackSubtypes",
			scan: func(src any) (string, error) {
				var v NotificationFeedbackSubtypes
				err := v.Scan(src)
				return string(v), err
			},
			valid: func(v string) bool { return NotificationFeedbackSubtypes(v).Valid() },
			all:   func() []string { return notificationFeedbackSubtypesValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullNotificationFeedbackSubtypes
				err := v.Scan(src)
				return string(v.NotificationFeedbackSubtypes), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullNotificationFeedbackSubtypes{NotificationFeedbackSubtypes: NotificationFeedbackSubtypes(v), Valid: valid}.Value()
			},
		},
		{
			name: "NotificationFeedbackTypes",
			scan: func(src any) (string, error) {
				var v NotificationFeedbackTypes
				err := v.Scan(src)
				return string(v), err
			},
			valid: func(v string) bool { return NotificationFeedbackTypes(v).Valid() },
			all:   func() []string { return notificationFeedbackTypesValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullNotificationFeedbackTypes
				err := v.Scan(src)
				return string(v.NotificationFeedbackTypes), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullNotificationFeedbackTypes{NotificationFeedbackTypes: NotificationFeedbackTypes(v), Valid: valid}.Value()
			},
		},
		{
			name:  "NotificationType",
			scan:  func(src any) (string, error) { var v NotificationType; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return NotificationType(v).Valid() },
			all:   func() []string { return notificationTypeValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullNotificationType
				err := v.Scan(src)
				return string(v.NotificationType), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullNotificationType{NotificationType: NotificationType(v), Valid: valid}.Value()
			},
		},
		{
			name:  "NotifyStatusType",
			scan:  func(src any) (string, error) { var v NotifyStatusType; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return NotifyStatusType(v).Valid() },
			all:   func() []string { return notifyStatusTypeValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullNotifyStatusType
				err := v.Scan(src)
				return string(v.NotifyStatusType), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullNotifyStatusType{NotifyStatusType: NotifyStatusType(v), Valid: valid}.Value()
			},
		},
		{
			name:  "PermissionTypes",
			scan:  func(src any) (string, error) { var v PermissionTypes; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return PermissionTypes(v).Valid() },
			all:   func() []string { return permissionTypesValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullPermissionTypes
				err := v.Scan(src)
				return string(v.PermissionTypes), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullPermissionTypes{PermissionTypes: PermissionTypes(v), Valid: valid}.Value()
			},
		},
		{
			name:  "RecipientType",
			scan:  func(src any) (string, error) { var v RecipientType; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return RecipientType(v).Valid() },
			all:   func() []string { return recipientTypeValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullRecipientType
				err := v.Scan(src)
				return string(v.RecipientType), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullRecipientType{RecipientType: RecipientType(v), Valid: valid}.Value()
			},
		},
		{
			name:  "SmsSendingVehicle",
			scan:  func(src any) (string, error) { var v SmsSendingVehicle; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return SmsSendingVehicle(v).Valid() },
			all:   func() []string { return smsSendingVehicleValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullSmsSendingVehicle
				err := v.Scan(src)
				return string(v.SmsSendingVehicle), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullSmsSendingVehicle{SmsSendingVehicle: SmsSendingVehicle(v), Valid: valid}.Value()
			},
		},
		{
			name:  "TemplateType",
			scan:  func(src any) (string, error) { var v TemplateType; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return TemplateType(v).Valid() },
			all:   func() []string { return templateTypeValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullTemplateType
				err := v.Scan(src)
				return string(v.TemplateType), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullTemplateType{TemplateType: TemplateType(v), Valid: valid}.Value()
			},
		},
		{
			name:  "VerifyCodeTypes",
			scan:  func(src any) (string, error) { var v VerifyCodeTypes; err := v.Scan(src); return string(v), err },
			valid: func(v string) bool { return VerifyCodeTypes(v).Valid() },
			all:   func() []string { return verifyCodeTypesValues() },
			nullScan: func(src any) (string, bool, error) {
				var v NullVerifyCodeTypes
				err := v.Scan(src)
				return string(v.VerifyCodeTypes), v.Valid, err
			},
			nullValue: func(v string, valid bool) (driver.Value, error) {
				return NullVerifyCodeTypes{VerifyCodeTypes: VerifyCodeTypes(v), Valid: valid}.Value()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			all := tc.all()
			if len(all) == 0 {
				t.Fatal("expected enum values")
			}

			first := all[0]

			if got, err := tc.scan(first); err != nil || got != first {
				t.Fatalf("scan(string) = (%q, %v), want (%q, nil)", got, err, first)
			}
			if got, err := tc.scan([]byte(first)); err != nil || got != first {
				t.Fatalf("scan([]byte) = (%q, %v), want (%q, nil)", got, err, first)
			}
			if _, err := tc.scan(123); err == nil {
				t.Fatal("expected unsupported scan type error")
			}

			if !tc.valid(first) {
				t.Fatalf("valid(%q) = false, want true", first)
			}
			if tc.valid("not-a-valid-value") {
				t.Fatal("expected invalid enum value")
			}
			for _, value := range all {
				if !tc.valid(value) {
					t.Fatalf("all() returned invalid value %q", value)
				}
			}

			if got, valid, err := tc.nullScan(nil); err != nil || got != "" || valid {
				t.Fatalf("nullScan(nil) = (%q, %t, %v), want (empty, false, nil)", got, valid, err)
			}
			if got, valid, err := tc.nullScan(first); err != nil || got != first || !valid {
				t.Fatalf("nullScan(value) = (%q, %t, %v), want (%q, true, nil)", got, valid, err, first)
			}

			if got, err := tc.nullValue(first, true); err != nil || got != first {
				t.Fatalf("nullValue(valid) = (%#v, %v), want (%q, nil)", got, err, first)
			}
			if got, err := tc.nullValue(first, false); err != nil || got != nil {
				t.Fatalf("nullValue(invalid) = (%#v, %v), want (nil, nil)", got, err)
			}
		})
	}
}

func TestQueries(t *testing.T) {
	db := &fakeDB{}
	queries := New(db)
	if queries.db != db {
		t.Fatal("New() did not retain db")
	}

	withTx := queries.WithTx(nil)
	if _, ok := withTx.db.(*sql.Tx); !ok {
		t.Fatalf("WithTx() db type = %T, want *sql.Tx", withTx.db)
	}

	if err := queries.Placeholder(context.Background()); err != nil {
		t.Fatalf("Placeholder() error = %v, want nil", err)
	}
	if db.query != placeholder {
		t.Fatalf("query = %q, want placeholder query", db.query)
	}

	db.err = errors.New("exec failed")
	if err := queries.Placeholder(context.Background()); !errors.Is(err, db.err) {
		t.Fatalf("Placeholder() error = %v, want %v", err, db.err)
	}
}

func invitedUsersStatusTypesValues() []string {
	values := AllInvitedUsersStatusTypesValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func jobStatusTypesValues() []string {
	values := AllJobStatusTypesValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func notificationFeedbackSubtypesValues() []string {
	values := AllNotificationFeedbackSubtypesValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func notificationFeedbackTypesValues() []string {
	values := AllNotificationFeedbackTypesValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func notificationTypeValues() []string {
	values := AllNotificationTypeValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func notifyStatusTypeValues() []string {
	values := AllNotifyStatusTypeValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func permissionTypesValues() []string {
	values := AllPermissionTypesValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func recipientTypeValues() []string {
	values := AllRecipientTypeValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func smsSendingVehicleValues() []string {
	values := AllSmsSendingVehicleValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func templateTypeValues() []string {
	values := AllTemplateTypeValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}

func verifyCodeTypesValues() []string {
	values := AllVerifyCodeTypesValues()
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}
