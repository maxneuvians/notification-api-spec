package services

import "context"

func (q *Queries) InsertServicesHistory(ctx context.Context, history *ServicesHistory) error {
	if history == nil {
		return nil
	}

	return q.InsertServicesHistoryRow(ctx, InsertServicesHistoryRowParams{
		ID:                      history.ID,
		Name:                    history.Name,
		CreatedAt:               history.CreatedAt,
		UpdatedAt:               history.UpdatedAt,
		Active:                  history.Active,
		MessageLimit:            history.MessageLimit,
		Restricted:              history.Restricted,
		EmailFrom:               history.EmailFrom,
		CreatedByID:             history.CreatedByID,
		Version:                 history.Version,
		ResearchMode:            history.ResearchMode,
		OrganisationType:        history.OrganisationType,
		PrefixSms:               history.PrefixSms,
		Crown:                   history.Crown,
		RateLimit:               history.RateLimit,
		ContactLink:             history.ContactLink,
		ConsentToResearch:       history.ConsentToResearch,
		VolumeEmail:             history.VolumeEmail,
		VolumeLetter:            history.VolumeLetter,
		VolumeSms:               history.VolumeSms,
		CountAsLive:             history.CountAsLive,
		GoLiveAt:                history.GoLiveAt,
		GoLiveUserID:            history.GoLiveUserID,
		OrganisationID:          history.OrganisationID,
		SendingDomain:           history.SendingDomain,
		DefaultBrandingIsFrench: history.DefaultBrandingIsFrench,
		SmsDailyLimit:           history.SmsDailyLimit,
		OrganisationNotes:       history.OrganisationNotes,
		SensitiveService:        history.SensitiveService,
		EmailAnnualLimit:        history.EmailAnnualLimit,
		SmsAnnualLimit:          history.SmsAnnualLimit,
		SuspendedByID:           history.SuspendedByID,
		SuspendedAt:             history.SuspendedAt,
	})
}
