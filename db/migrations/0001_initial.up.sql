CREATE TYPE public.invited_users_status_types AS ENUM (
    'pending',
    'accepted',
    'cancelled'
);

CREATE TYPE public.job_status_types AS ENUM (
    'pending',
    'in progress',
    'finished',
    'sending limits exceeded'
);

CREATE TYPE public.notification_feedback_subtypes AS ENUM (
    'general',
    'no-email',
    'suppressed',
    'on-account-suppression-list',
    'mailbox-full',
    'message-too-large',
    'content-rejected',
    'attachment-rejected',
    'unknown-bounce-subtype'
);

CREATE TYPE public.notification_feedback_types AS ENUM (
    'hard-bounce',
    'soft-bounce',
    'unknown-bounce'
);

CREATE TYPE public.notification_type AS ENUM (
    'email',
    'sms',
    'letter'
);

CREATE TYPE public.notify_status_type AS ENUM (
    'created',
    'sending',
    'delivered',
    'pending',
    'failed',
    'technical-failure',
    'temporary-failure',
    'permanent-failure',
    'sent'
);

CREATE TYPE public.permission_types AS ENUM (
    'manage_users',
    'manage_templates',
    'manage_settings',
    'send_texts',
    'send_emails',
    'send_letters',
    'manage_api_keys',
    'platform_admin',
    'view_activity'
);

CREATE TYPE public.recipient_type AS ENUM (
    'mobile',
    'email'
);

CREATE TYPE public.sms_sending_vehicle AS ENUM (
    'short_code',
    'long_code'
);

CREATE TYPE public.template_type AS ENUM (
    'sms',
    'email',
    'letter'
);

CREATE TYPE public.verify_code_types AS ENUM (
    'email',
    'sms'
);

CREATE TABLE public.alembic_version (
    version_num character varying(32) NOT NULL
);

ALTER TABLE public.alembic_version OWNER TO postgres;

CREATE TABLE public.annual_billing (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    financial_year_start integer NOT NULL,
    free_sms_fragment_limit integer NOT NULL,
    updated_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL
);

ALTER TABLE public.annual_billing OWNER TO postgres;

CREATE TABLE public.annual_limits_data (
    service_id uuid NOT NULL,
    time_period character varying NOT NULL,
    annual_email_limit bigint NOT NULL,
    annual_sms_limit bigint NOT NULL,
    notification_type character varying NOT NULL,
    notification_count bigint NOT NULL
);

ALTER TABLE public.annual_limits_data OWNER TO postgres;

CREATE TABLE public.api_keys (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    secret character varying(255) NOT NULL,
    service_id uuid NOT NULL,
    expiry_date timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    created_by_id uuid NOT NULL,
    updated_at timestamp without time zone,
    version integer NOT NULL,
    key_type character varying(255) NOT NULL,
    compromised_key_info jsonb,
    last_used_timestamp timestamp without time zone
);

ALTER TABLE public.api_keys OWNER TO postgres;

CREATE TABLE public.api_keys_history (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    secret character varying(255) NOT NULL,
    service_id uuid NOT NULL,
    expiry_date timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    created_by_id uuid NOT NULL,
    version integer NOT NULL,
    key_type character varying(255) NOT NULL,
    compromised_key_info jsonb,
    last_used_timestamp timestamp without time zone
);

ALTER TABLE public.api_keys_history OWNER TO postgres;

CREATE TABLE public.auth_type (
    name character varying NOT NULL
);

ALTER TABLE public.auth_type OWNER TO postgres;

CREATE TABLE public.branding_type (
    name character varying(255) NOT NULL
);

ALTER TABLE public.branding_type OWNER TO postgres;

CREATE TABLE public.complaints (
    id uuid NOT NULL,
    notification_id uuid NOT NULL,
    service_id uuid NOT NULL,
    ses_feedback_id text,
    complaint_type text,
    complaint_date timestamp without time zone,
    created_at timestamp without time zone NOT NULL
);

ALTER TABLE public.complaints OWNER TO postgres;

CREATE TABLE public.daily_sorted_letter (
    id uuid NOT NULL,
    billing_day date NOT NULL,
    unsorted_count integer NOT NULL,
    sorted_count integer NOT NULL,
    updated_at timestamp without time zone,
    file_name character varying
);

ALTER TABLE public.daily_sorted_letter OWNER TO postgres;

CREATE TABLE public.dm_datetime (
    bst_date date NOT NULL,
    year integer NOT NULL,
    month integer NOT NULL,
    month_name character varying NOT NULL,
    day integer NOT NULL,
    bst_day integer NOT NULL,
    day_of_year integer NOT NULL,
    week_day_name character varying NOT NULL,
    calendar_week integer NOT NULL,
    quartal character varying NOT NULL,
    year_quartal character varying NOT NULL,
    year_month character varying NOT NULL,
    year_calendar_week character varying NOT NULL,
    financial_year integer NOT NULL,
    utc_daytime_start timestamp without time zone NOT NULL,
    utc_daytime_end timestamp without time zone NOT NULL
);

ALTER TABLE public.dm_datetime OWNER TO postgres;

CREATE TABLE public.domain (
    domain character varying(255) NOT NULL,
    organisation_id uuid NOT NULL
);

ALTER TABLE public.domain OWNER TO postgres;

CREATE TABLE public.email_branding (
    id uuid NOT NULL,
    colour character varying(7),
    logo character varying(255),
    name character varying(255) NOT NULL,
    text character varying(255),
    brand_type character varying(255) NOT NULL,
    organisation_id uuid,
    alt_text_en character varying,
    alt_text_fr character varying,
    created_by_id uuid NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_by_id uuid,
    updated_at timestamp without time zone DEFAULT now()
);

ALTER TABLE public.email_branding OWNER TO postgres;

CREATE TABLE public.events (
    id uuid NOT NULL,
    event_type character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    data json NOT NULL
);

ALTER TABLE public.events OWNER TO postgres;

CREATE TABLE public.fido2_keys (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    name character varying NOT NULL,
    key text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone
);

ALTER TABLE public.fido2_keys OWNER TO postgres;

CREATE TABLE public.fido2_sessions (
    user_id uuid NOT NULL,
    session text NOT NULL,
    created_at timestamp without time zone NOT NULL
);

ALTER TABLE public.fido2_sessions OWNER TO postgres;

CREATE TABLE public.ft_billing (
    bst_date date NOT NULL,
    template_id uuid NOT NULL,
    service_id uuid NOT NULL,
    notification_type text NOT NULL,
    provider text NOT NULL,
    rate_multiplier integer NOT NULL,
    international boolean NOT NULL,
    rate numeric NOT NULL,
    billable_units integer,
    notifications_sent integer,
    updated_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL,
    postage character varying NOT NULL,
    sms_sending_vehicle public.sms_sending_vehicle DEFAULT 'long_code'::public.sms_sending_vehicle NOT NULL,
    billing_total numeric(16,8)
);

ALTER TABLE public.ft_billing OWNER TO postgres;

CREATE TABLE public.ft_notification_status (
    bst_date date NOT NULL,
    template_id uuid NOT NULL,
    service_id uuid NOT NULL,
    job_id uuid NOT NULL,
    notification_type text NOT NULL,
    key_type text NOT NULL,
    notification_status text NOT NULL,
    notification_count integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    billable_units integer NOT NULL
);

ALTER TABLE public.ft_notification_status OWNER TO postgres;

CREATE TABLE public.inbound_numbers (
    id uuid NOT NULL,
    number character varying(11) NOT NULL,
    provider character varying NOT NULL,
    service_id uuid,
    active boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone
);

ALTER TABLE public.inbound_numbers OWNER TO postgres;

CREATE TABLE public.inbound_sms (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    content character varying NOT NULL,
    notify_number character varying NOT NULL,
    user_number character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    provider_date timestamp without time zone,
    provider_reference character varying,
    provider character varying NOT NULL
);

ALTER TABLE public.inbound_sms OWNER TO postgres;

CREATE TABLE public.invite_status_type (
    name character varying NOT NULL
);

ALTER TABLE public.invite_status_type OWNER TO postgres;

CREATE TABLE public.invited_organisation_users (
    id uuid NOT NULL,
    email_address character varying(255) NOT NULL,
    invited_by_id uuid NOT NULL,
    organisation_id uuid NOT NULL,
    created_at timestamp without time zone NOT NULL,
    status character varying NOT NULL
);

ALTER TABLE public.invited_organisation_users OWNER TO postgres;

CREATE TABLE public.invited_users (
    id uuid NOT NULL,
    email_address character varying(255) NOT NULL,
    user_id uuid NOT NULL,
    service_id uuid,
    created_at timestamp without time zone NOT NULL,
    status public.invited_users_status_types NOT NULL,
    permissions character varying NOT NULL,
    auth_type character varying DEFAULT 'sms_auth'::character varying NOT NULL,
    folder_permissions jsonb NOT NULL
);

ALTER TABLE public.invited_users OWNER TO postgres;

CREATE TABLE public.job_status (
    name character varying(255) NOT NULL
);

ALTER TABLE public.job_status OWNER TO postgres;

CREATE TABLE public.jobs (
    id uuid NOT NULL,
    original_file_name character varying NOT NULL,
    service_id uuid NOT NULL,
    template_id uuid,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    notification_count integer NOT NULL,
    notifications_sent integer NOT NULL,
    processing_started timestamp without time zone,
    processing_finished timestamp without time zone,
    created_by_id uuid,
    template_version integer NOT NULL,
    notifications_delivered integer NOT NULL,
    notifications_failed integer NOT NULL,
    job_status character varying(255) NOT NULL,
    scheduled_for timestamp without time zone,
    archived boolean DEFAULT false NOT NULL,
    api_key_id uuid,
    sender_id uuid
);

ALTER TABLE public.jobs OWNER TO postgres;

CREATE TABLE public.key_types (
    name character varying(255) NOT NULL
);

ALTER TABLE public.key_types OWNER TO postgres;

CREATE TABLE public.letter_branding (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    filename character varying(255) NOT NULL
);

ALTER TABLE public.letter_branding OWNER TO postgres;

CREATE TABLE public.letter_rates (
    id uuid NOT NULL,
    start_date timestamp without time zone NOT NULL,
    end_date timestamp without time zone,
    sheet_count integer NOT NULL,
    rate numeric NOT NULL,
    crown boolean NOT NULL,
    post_class character varying NOT NULL
);

ALTER TABLE public.letter_rates OWNER TO postgres;

CREATE TABLE public.login_events (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    data jsonb NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone
);

ALTER TABLE public.login_events OWNER TO postgres;

CREATE TABLE public.monthly_notification_stats_summary (
    month text NOT NULL,
    service_id uuid NOT NULL,
    notification_type text NOT NULL,
    notification_count integer NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);

ALTER TABLE public.monthly_notification_stats_summary OWNER TO postgres;

CREATE TABLE public.notification_history (
    id uuid NOT NULL,
    job_id uuid,
    job_row_number integer,
    service_id uuid,
    template_id uuid,
    template_version integer NOT NULL,
    api_key_id uuid,
    key_type character varying NOT NULL,
    notification_type public.notification_type NOT NULL,
    created_at timestamp without time zone NOT NULL,
    sent_at timestamp without time zone,
    sent_by character varying,
    updated_at timestamp without time zone,
    reference character varying,
    billable_units integer NOT NULL,
    client_reference character varying,
    international boolean,
    phone_prefix character varying,
    rate_multiplier numeric,
    notification_status text,
    created_by_id uuid,
    postage character varying,
    queue_name text,
    feedback_type public.notification_feedback_types,
    feedback_subtype public.notification_feedback_subtypes,
    ses_feedback_id character varying,
    ses_feedback_date timestamp without time zone,
    sms_total_message_price double precision,
    sms_total_carrier_fee double precision,
    sms_iso_country_code character varying,
    sms_carrier_name character varying,
    sms_message_encoding character varying,
    sms_origination_phone_number character varying,
    feedback_reason character varying(255),
    CONSTRAINT chk_notification_history_postage_null CHECK (
CASE
    WHEN (notification_type = 'letter'::public.notification_type) THEN ((postage IS NOT NULL) AND ((postage)::text = ANY ((ARRAY['first'::character varying, 'second'::character varying])::text[])))
    ELSE (postage IS NULL)
END)
);

ALTER TABLE public.notification_history OWNER TO postgres;

CREATE TABLE public.notification_status_types (
    name character varying NOT NULL
);

ALTER TABLE public.notification_status_types OWNER TO postgres;

CREATE TABLE public.notifications (
    id uuid NOT NULL,
    "to" character varying NOT NULL,
    job_id uuid,
    service_id uuid,
    template_id uuid,
    created_at timestamp without time zone NOT NULL,
    sent_at timestamp without time zone,
    sent_by character varying,
    updated_at timestamp without time zone,
    reference character varying,
    template_version integer NOT NULL,
    job_row_number integer,
    _personalisation character varying,
    api_key_id uuid,
    key_type character varying(255) NOT NULL,
    notification_type public.notification_type NOT NULL,
    billable_units integer NOT NULL,
    client_reference character varying,
    international boolean,
    phone_prefix character varying,
    rate_multiplier numeric,
    notification_status text,
    normalised_to character varying,
    created_by_id uuid,
    reply_to_text character varying,
    postage character varying,
    provider_response text,
    queue_name text,
    feedback_type public.notification_feedback_types,
    feedback_subtype public.notification_feedback_subtypes,
    ses_feedback_id character varying,
    ses_feedback_date timestamp without time zone,
    sms_total_message_price double precision,
    sms_total_carrier_fee double precision,
    sms_iso_country_code character varying,
    sms_carrier_name character varying,
    sms_message_encoding character varying,
    sms_origination_phone_number character varying,
    feedback_reason character varying(255),
    CONSTRAINT chk_notifications_postage_null CHECK (
CASE
    WHEN (notification_type = 'letter'::public.notification_type) THEN ((postage IS NOT NULL) AND ((postage)::text = ANY ((ARRAY['first'::character varying, 'second'::character varying])::text[])))
    ELSE (postage IS NULL)
END)
);

ALTER TABLE public.notifications OWNER TO postgres;

CREATE TABLE public.organisation (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    active boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    email_branding_id uuid,
    letter_branding_id uuid,
    agreement_signed boolean,
    agreement_signed_at timestamp without time zone,
    agreement_signed_by_id uuid,
    agreement_signed_version double precision,
    crown boolean,
    organisation_type character varying(255),
    request_to_go_live_notes text,
    agreement_signed_on_behalf_of_email_address character varying(255),
    agreement_signed_on_behalf_of_name character varying(255),
    default_branding_is_french boolean DEFAULT false
);

ALTER TABLE public.organisation OWNER TO postgres;

CREATE TABLE public.organisation_types (
    name character varying(255) NOT NULL,
    is_crown boolean,
    annual_free_sms_fragment_limit bigint NOT NULL
);

ALTER TABLE public.organisation_types OWNER TO postgres;

CREATE TABLE public.permissions (
    id uuid NOT NULL,
    service_id uuid,
    user_id uuid NOT NULL,
    permission public.permission_types NOT NULL,
    created_at timestamp without time zone NOT NULL
);

ALTER TABLE public.permissions OWNER TO postgres;

CREATE TABLE public.provider_details (
    id uuid NOT NULL,
    display_name character varying NOT NULL,
    identifier character varying NOT NULL,
    priority integer NOT NULL,
    notification_type public.notification_type NOT NULL,
    active boolean NOT NULL,
    updated_at timestamp without time zone,
    version integer NOT NULL,
    created_by_id uuid,
    supports_international boolean DEFAULT false NOT NULL
);

ALTER TABLE public.provider_details OWNER TO postgres;

CREATE TABLE public.provider_details_history (
    id uuid NOT NULL,
    display_name character varying NOT NULL,
    identifier character varying NOT NULL,
    priority integer NOT NULL,
    notification_type public.notification_type NOT NULL,
    active boolean NOT NULL,
    version integer NOT NULL,
    updated_at timestamp without time zone,
    created_by_id uuid,
    supports_international boolean DEFAULT false NOT NULL
);

ALTER TABLE public.provider_details_history OWNER TO postgres;

CREATE TABLE public.provider_rates (
    id uuid NOT NULL,
    valid_from timestamp without time zone NOT NULL,
    rate numeric NOT NULL,
    provider_id uuid NOT NULL
);

ALTER TABLE public.provider_rates OWNER TO postgres;

CREATE TABLE public.rates (
    id uuid NOT NULL,
    valid_from timestamp without time zone NOT NULL,
    rate numeric NOT NULL,
    notification_type public.notification_type NOT NULL,
    sms_sending_vehicle public.sms_sending_vehicle DEFAULT 'long_code'::public.sms_sending_vehicle NOT NULL
);

ALTER TABLE public.rates OWNER TO postgres;

CREATE TABLE public.reports (
    id uuid NOT NULL,
    report_type character varying(255) NOT NULL,
    requested_at timestamp without time zone NOT NULL,
    completed_at timestamp without time zone,
    expires_at timestamp without time zone,
    requesting_user_id uuid,
    service_id uuid NOT NULL,
    job_id uuid,
    url character varying(2000),
    status character varying(255) NOT NULL,
    language character varying(2)
);

ALTER TABLE public.reports OWNER TO postgres;

CREATE TABLE public.scheduled_notifications (
    id uuid NOT NULL,
    notification_id uuid NOT NULL,
    scheduled_for timestamp without time zone NOT NULL,
    pending boolean NOT NULL
);

ALTER TABLE public.scheduled_notifications OWNER TO postgres;

CREATE TABLE public.service_callback_api (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    url character varying NOT NULL,
    bearer_token character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    updated_by_id uuid NOT NULL,
    version integer NOT NULL,
    callback_type character varying,
    is_suspended boolean,
    suspended_at timestamp without time zone
);

ALTER TABLE public.service_callback_api OWNER TO postgres;

CREATE TABLE public.service_callback_api_history (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    url character varying NOT NULL,
    bearer_token character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    updated_by_id uuid NOT NULL,
    version integer NOT NULL,
    callback_type character varying,
    is_suspended boolean,
    suspended_at timestamp without time zone
);

ALTER TABLE public.service_callback_api_history OWNER TO postgres;

CREATE TABLE public.service_callback_type (
    name character varying NOT NULL
);

ALTER TABLE public.service_callback_type OWNER TO postgres;

CREATE TABLE public.service_data_retention (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    notification_type public.notification_type NOT NULL,
    days_of_retention integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone
);

ALTER TABLE public.service_data_retention OWNER TO postgres;

CREATE TABLE public.service_email_branding (
    service_id uuid NOT NULL,
    email_branding_id uuid NOT NULL
);

ALTER TABLE public.service_email_branding OWNER TO postgres;

CREATE TABLE public.service_email_reply_to (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    email_address text NOT NULL,
    is_default boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    archived boolean DEFAULT false NOT NULL
);

ALTER TABLE public.service_email_reply_to OWNER TO postgres;

CREATE TABLE public.service_inbound_api (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    url character varying NOT NULL,
    bearer_token character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    updated_by_id uuid NOT NULL,
    version integer NOT NULL
);

ALTER TABLE public.service_inbound_api OWNER TO postgres;

CREATE TABLE public.service_inbound_api_history (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    url character varying NOT NULL,
    bearer_token character varying NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    updated_by_id uuid NOT NULL,
    version integer NOT NULL
);

ALTER TABLE public.service_inbound_api_history OWNER TO postgres;

CREATE TABLE public.service_letter_branding (
    service_id uuid NOT NULL,
    letter_branding_id uuid NOT NULL
);

ALTER TABLE public.service_letter_branding OWNER TO postgres;

CREATE TABLE public.service_letter_contacts (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    contact_block text NOT NULL,
    is_default boolean NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    archived boolean DEFAULT false NOT NULL
);

ALTER TABLE public.service_letter_contacts OWNER TO postgres;

CREATE TABLE public.service_permission_types (
    name character varying(255) NOT NULL
);

ALTER TABLE public.service_permission_types OWNER TO postgres;

CREATE TABLE public.service_permissions (
    service_id uuid NOT NULL,
    permission character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL
);

ALTER TABLE public.service_permissions OWNER TO postgres;

CREATE TABLE public.service_safelist (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    recipient_type public.recipient_type NOT NULL,
    recipient character varying(255) NOT NULL,
    created_at timestamp without time zone
);

ALTER TABLE public.service_safelist OWNER TO postgres;

CREATE TABLE public.service_sms_senders (
    id uuid NOT NULL,
    sms_sender character varying(11) NOT NULL,
    service_id uuid NOT NULL,
    is_default boolean NOT NULL,
    inbound_number_id uuid,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    archived boolean DEFAULT false NOT NULL
);

ALTER TABLE public.service_sms_senders OWNER TO postgres;

CREATE TABLE public.services (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    active boolean NOT NULL,
    message_limit bigint NOT NULL,
    restricted boolean NOT NULL,
    email_from text NOT NULL,
    created_by_id uuid NOT NULL,
    version integer NOT NULL,
    research_mode boolean NOT NULL,
    organisation_type character varying(255),
    prefix_sms boolean NOT NULL,
    crown boolean,
    rate_limit integer DEFAULT 1000 NOT NULL,
    contact_link character varying(255),
    consent_to_research boolean,
    volume_email integer,
    volume_letter integer,
    volume_sms integer,
    count_as_live boolean DEFAULT true NOT NULL,
    go_live_at timestamp without time zone,
    go_live_user_id uuid,
    organisation_id uuid,
    sending_domain text,
    default_branding_is_french boolean DEFAULT false,
    sms_daily_limit bigint NOT NULL,
    organisation_notes character varying,
    sensitive_service boolean,
    email_annual_limit bigint DEFAULT 20000000 NOT NULL,
    sms_annual_limit bigint DEFAULT 100000 NOT NULL,
    suspended_by_id uuid,
    suspended_at timestamp without time zone
);

ALTER TABLE public.services OWNER TO postgres;

CREATE TABLE public.services_history (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    active boolean NOT NULL,
    message_limit bigint NOT NULL,
    restricted boolean NOT NULL,
    email_from text NOT NULL,
    created_by_id uuid NOT NULL,
    version integer NOT NULL,
    research_mode boolean NOT NULL,
    organisation_type character varying(255),
    prefix_sms boolean,
    crown boolean,
    rate_limit integer DEFAULT 1000 NOT NULL,
    contact_link character varying(255),
    consent_to_research boolean,
    volume_email integer,
    volume_letter integer,
    volume_sms integer,
    count_as_live boolean DEFAULT true NOT NULL,
    go_live_at timestamp without time zone,
    go_live_user_id uuid,
    organisation_id uuid,
    sending_domain text,
    default_branding_is_french boolean DEFAULT false,
    sms_daily_limit bigint NOT NULL,
    organisation_notes character varying,
    sensitive_service boolean,
    email_annual_limit bigint DEFAULT '20000000'::bigint NOT NULL,
    sms_annual_limit bigint DEFAULT '100000'::bigint NOT NULL,
    suspended_by_id uuid,
    suspended_at timestamp without time zone
);

ALTER TABLE public.services_history OWNER TO postgres;

CREATE TABLE public.template_categories (
    id uuid NOT NULL,
    name_en character varying(255) NOT NULL,
    name_fr character varying(255) NOT NULL,
    description_en character varying(255),
    description_fr character varying(255),
    sms_process_type character varying(255) NOT NULL,
    email_process_type character varying(255) NOT NULL,
    hidden boolean NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now(),
    sms_sending_vehicle public.sms_sending_vehicle DEFAULT 'long_code'::public.sms_sending_vehicle NOT NULL,
    created_by_id uuid NOT NULL,
    updated_by_id uuid
);

ALTER TABLE public.template_categories OWNER TO postgres;

CREATE TABLE public.template_folder (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    name character varying NOT NULL,
    parent_id uuid
);

ALTER TABLE public.template_folder OWNER TO postgres;

CREATE TABLE public.template_folder_map (
    template_id uuid NOT NULL,
    template_folder_id uuid NOT NULL
);

ALTER TABLE public.template_folder_map OWNER TO postgres;

CREATE TABLE public.template_process_type (
    name character varying(255) NOT NULL
);

ALTER TABLE public.template_process_type OWNER TO postgres;

CREATE TABLE public.template_redacted (
    template_id uuid NOT NULL,
    redact_personalisation boolean NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    updated_by_id uuid NOT NULL
);

ALTER TABLE public.template_redacted OWNER TO postgres;

CREATE TABLE public.templates (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    template_type public.template_type NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    content text NOT NULL,
    service_id uuid NOT NULL,
    subject text,
    created_by_id uuid NOT NULL,
    version integer NOT NULL,
    archived boolean NOT NULL,
    process_type character varying(255),
    service_letter_contact_id uuid,
    hidden boolean NOT NULL,
    postage character varying,
    template_category_id uuid,
    text_direction_rtl boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_templates_postage CHECK (
CASE
    WHEN (template_type = 'letter'::public.template_type) THEN ((postage IS NOT NULL) AND ((postage)::text = ANY ((ARRAY['first'::character varying, 'second'::character varying])::text[])))
    ELSE (postage IS NULL)
END)
);

ALTER TABLE public.templates OWNER TO postgres;

CREATE TABLE public.templates_history (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    template_type public.template_type NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    content text NOT NULL,
    service_id uuid NOT NULL,
    subject text,
    created_by_id uuid NOT NULL,
    version integer NOT NULL,
    archived boolean NOT NULL,
    process_type character varying(255),
    service_letter_contact_id uuid,
    hidden boolean NOT NULL,
    postage character varying,
    template_category_id uuid,
    text_direction_rtl boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_templates_history_postage CHECK (
CASE
    WHEN (template_type = 'letter'::public.template_type) THEN ((postage IS NOT NULL) AND ((postage)::text = ANY ((ARRAY['first'::character varying, 'second'::character varying])::text[])))
    ELSE (postage IS NULL)
END)
);

ALTER TABLE public.templates_history OWNER TO postgres;

CREATE TABLE public.user_folder_permissions (
    user_id uuid NOT NULL,
    template_folder_id uuid NOT NULL,
    service_id uuid NOT NULL
);

ALTER TABLE public.user_folder_permissions OWNER TO postgres;

CREATE TABLE public.user_to_organisation (
    user_id uuid,
    organisation_id uuid
);

ALTER TABLE public.user_to_organisation OWNER TO postgres;

CREATE TABLE public.user_to_service (
    user_id uuid,
    service_id uuid
);

ALTER TABLE public.user_to_service OWNER TO postgres;

CREATE TABLE public.users (
    id uuid NOT NULL,
    name character varying NOT NULL,
    email_address character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone,
    _password character varying NOT NULL,
    mobile_number character varying,
    password_changed_at timestamp without time zone NOT NULL,
    logged_in_at timestamp without time zone,
    failed_login_count integer NOT NULL,
    state character varying NOT NULL,
    platform_admin boolean NOT NULL,
    current_session_id uuid,
    auth_type character varying DEFAULT 'sms_auth'::character varying NOT NULL,
    blocked boolean DEFAULT false NOT NULL,
    additional_information jsonb,
    password_expired boolean DEFAULT false NOT NULL,
    verified_phonenumber boolean DEFAULT false,
    default_editor_is_rte boolean DEFAULT false NOT NULL,
    CONSTRAINT ck_users_mobile_or_email_auth CHECK ((((auth_type)::text = ANY ((ARRAY['email_auth'::character varying, 'security_key_auth'::character varying])::text[])) OR (mobile_number IS NOT NULL)))
);

ALTER TABLE public.users OWNER TO postgres;

CREATE TABLE public.verify_codes (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    _code character varying NOT NULL,
    code_type public.verify_code_types NOT NULL,
    expiry_datetime timestamp without time zone NOT NULL,
    code_used boolean,
    created_at timestamp without time zone NOT NULL
);

ALTER TABLE public.verify_codes OWNER TO postgres;

ALTER TABLE ONLY public.alembic_version
    ADD CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num);

ALTER TABLE ONLY public.annual_billing
    ADD CONSTRAINT annual_billing_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.api_keys_history
    ADD CONSTRAINT api_keys_history_pkey PRIMARY KEY (id, version);

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_secret_key UNIQUE (secret);

ALTER TABLE ONLY public.auth_type
    ADD CONSTRAINT auth_type_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.branding_type
    ADD CONSTRAINT branding_type_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.complaints
    ADD CONSTRAINT complaints_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.daily_sorted_letter
    ADD CONSTRAINT daily_sorted_letter_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.dm_datetime
    ADD CONSTRAINT dm_datetime_pkey PRIMARY KEY (bst_date);

ALTER TABLE ONLY public.domain
    ADD CONSTRAINT domain_pkey PRIMARY KEY (domain);

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.fido2_keys
    ADD CONSTRAINT fido2_keys_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.fido2_sessions
    ADD CONSTRAINT fido2_sessions_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY public.ft_billing
    ADD CONSTRAINT ft_billing_pkey PRIMARY KEY (bst_date, template_id, service_id, notification_type, provider, rate_multiplier, international, rate, postage, sms_sending_vehicle);

ALTER TABLE ONLY public.ft_notification_status
    ADD CONSTRAINT ft_notification_status_pkey PRIMARY KEY (bst_date, template_id, service_id, job_id, notification_type, key_type, notification_status);

ALTER TABLE ONLY public.inbound_numbers
    ADD CONSTRAINT inbound_numbers_number_key UNIQUE (number);

ALTER TABLE ONLY public.inbound_numbers
    ADD CONSTRAINT inbound_numbers_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.inbound_sms
    ADD CONSTRAINT inbound_sms_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.invite_status_type
    ADD CONSTRAINT invite_status_type_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT ix_id_service_id UNIQUE (id, service_id);

ALTER TABLE ONLY public.job_status
    ADD CONSTRAINT job_status_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.key_types
    ADD CONSTRAINT key_types_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.letter_branding
    ADD CONSTRAINT letter_branding_filename_key UNIQUE (filename);

ALTER TABLE ONLY public.letter_branding
    ADD CONSTRAINT letter_branding_name_key UNIQUE (name);

ALTER TABLE ONLY public.letter_branding
    ADD CONSTRAINT letter_branding_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.letter_rates
    ADD CONSTRAINT letter_rates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.login_events
    ADD CONSTRAINT login_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.monthly_notification_stats_summary
    ADD CONSTRAINT monthly_notification_stats_pkey PRIMARY KEY (month, service_id, notification_type);

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.notification_status_types
    ADD CONSTRAINT notification_status_types_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT organisation_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.organisation_types
    ADD CONSTRAINT organisation_types_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.provider_details_history
    ADD CONSTRAINT provider_details_history_pkey PRIMARY KEY (id, version);

ALTER TABLE ONLY public.provider_details
    ADD CONSTRAINT provider_details_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.provider_rates
    ADD CONSTRAINT provider_rates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.rates
    ADD CONSTRAINT rates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.scheduled_notifications
    ADD CONSTRAINT scheduled_notifications_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.service_callback_api_history
    ADD CONSTRAINT service_callback_api_history_pkey PRIMARY KEY (id, version);

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.service_callback_type
    ADD CONSTRAINT service_callback_type_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.service_data_retention
    ADD CONSTRAINT service_data_retention_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.service_email_reply_to
    ADD CONSTRAINT service_email_reply_to_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.service_inbound_api_history
    ADD CONSTRAINT service_inbound_api_history_pkey PRIMARY KEY (id, version);

ALTER TABLE ONLY public.service_inbound_api
    ADD CONSTRAINT service_inbound_api_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.service_letter_branding
    ADD CONSTRAINT service_letter_branding_pkey PRIMARY KEY (service_id);

ALTER TABLE ONLY public.service_letter_contacts
    ADD CONSTRAINT service_letter_contacts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.service_permission_types
    ADD CONSTRAINT service_permission_types_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.service_permissions
    ADD CONSTRAINT service_permissions_pkey PRIMARY KEY (service_id, permission);

ALTER TABLE ONLY public.service_sms_senders
    ADD CONSTRAINT service_sms_senders_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.service_safelist
    ADD CONSTRAINT service_whitelist_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_email_from_key UNIQUE (email_from);

ALTER TABLE ONLY public.services_history
    ADD CONSTRAINT services_history_pkey PRIMARY KEY (id, version);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_name_key UNIQUE (name);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_name_en_key UNIQUE (name_en);

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_name_fr_key UNIQUE (name_fr);

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.template_folder_map
    ADD CONSTRAINT template_folder_map_pkey PRIMARY KEY (template_id);

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT template_folder_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.template_process_type
    ADD CONSTRAINT template_process_type_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.template_redacted
    ADD CONSTRAINT template_redacted_pkey PRIMARY KEY (template_id);

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_pkey PRIMARY KEY (id, version);

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.daily_sorted_letter
    ADD CONSTRAINT uix_file_name_billing_day UNIQUE (file_name, billing_day);

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT uix_service_callback_type UNIQUE (service_id, callback_type);

ALTER TABLE ONLY public.service_data_retention
    ADD CONSTRAINT uix_service_data_retention UNIQUE (service_id, notification_type);

ALTER TABLE ONLY public.service_email_branding
    ADD CONSTRAINT uix_service_email_branding_one_per_service PRIMARY KEY (service_id);

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT uix_service_user_permission UNIQUE (service_id, user_id, permission);

ALTER TABLE ONLY public.user_to_organisation
    ADD CONSTRAINT uix_user_to_organisation UNIQUE (user_id, organisation_id);

ALTER TABLE ONLY public.user_to_service
    ADD CONSTRAINT uix_user_to_service UNIQUE (user_id, service_id);

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT uq_email_branding_name UNIQUE (name);

ALTER TABLE ONLY public.annual_limits_data
    ADD CONSTRAINT uq_service_id_notification_type_time_period UNIQUE (service_id, notification_type, time_period);

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_pkey PRIMARY KEY (user_id, template_folder_id, service_id);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.verify_codes
    ADD CONSTRAINT verify_codes_pkey PRIMARY KEY (id);

CREATE INDEX ix_annual_billing_service_id ON public.annual_billing USING btree (service_id);

CREATE INDEX ix_api_keys_created_by_id ON public.api_keys USING btree (created_by_id);

CREATE INDEX ix_api_keys_history_created_by_id ON public.api_keys_history USING btree (created_by_id);

CREATE INDEX ix_api_keys_history_key_type ON public.api_keys_history USING btree (key_type);

CREATE INDEX ix_api_keys_history_service_id ON public.api_keys_history USING btree (service_id);

CREATE INDEX ix_api_keys_key_type ON public.api_keys USING btree (key_type);

CREATE INDEX ix_api_keys_service_id ON public.api_keys USING btree (service_id);

CREATE INDEX ix_complaints_notification_id ON public.complaints USING btree (notification_id);

CREATE INDEX ix_complaints_service_id ON public.complaints USING btree (service_id);

CREATE INDEX ix_daily_sorted_letter_billing_day ON public.daily_sorted_letter USING btree (billing_day);

CREATE INDEX ix_daily_sorted_letter_file_name ON public.daily_sorted_letter USING btree (file_name);

CREATE INDEX ix_dm_datetime_bst_date ON public.dm_datetime USING btree (bst_date);

CREATE INDEX ix_dm_datetime_yearmonth ON public.dm_datetime USING btree (year, month);

CREATE INDEX ix_email_branding_brand_type ON public.email_branding USING btree (brand_type);

CREATE INDEX ix_email_branding_created_by_id ON public.email_branding USING btree (created_by_id);

CREATE INDEX ix_email_branding_organisation_id ON public.email_branding USING btree (organisation_id);

CREATE INDEX ix_email_branding_updated_by_id ON public.email_branding USING btree (updated_by_id);

CREATE INDEX ix_fido2_keys_user_id ON public.fido2_keys USING btree (user_id);

CREATE INDEX ix_ft_billing_bst_date ON public.ft_billing USING btree (bst_date);

CREATE INDEX ix_ft_billing_service_id ON public.ft_billing USING btree (service_id);

CREATE INDEX ix_ft_billing_template_id ON public.ft_billing USING btree (template_id);

CREATE INDEX ix_ft_notification_service_bst ON public.ft_notification_status USING btree (service_id, bst_date);

CREATE INDEX ix_ft_notification_status_bst_date ON public.ft_notification_status USING btree (bst_date);

CREATE INDEX ix_ft_notification_status_job_id ON public.ft_notification_status USING btree (job_id);

CREATE INDEX ix_ft_notification_status_service_id ON public.ft_notification_status USING btree (service_id);

CREATE INDEX ix_ft_notification_status_stats_lookup ON public.ft_notification_status USING btree (bst_date, notification_status, key_type) INCLUDE (notification_type, notification_count);

CREATE INDEX ix_ft_notification_status_template_id ON public.ft_notification_status USING btree (template_id);

CREATE INDEX ix_inbound_sms_service_id ON public.inbound_sms USING btree (service_id);

CREATE INDEX ix_inbound_sms_user_number ON public.inbound_sms USING btree (user_number);

CREATE INDEX ix_invited_users_auth_type ON public.invited_users USING btree (auth_type);

CREATE INDEX ix_invited_users_service_id ON public.invited_users USING btree (service_id);

CREATE INDEX ix_invited_users_user_id ON public.invited_users USING btree (user_id);

CREATE INDEX ix_jobs_api_key_id ON public.jobs USING btree (api_key_id);

CREATE INDEX ix_jobs_created_at ON public.jobs USING btree (created_at);

CREATE INDEX ix_jobs_created_by_id ON public.jobs USING btree (created_by_id);

CREATE INDEX ix_jobs_job_status ON public.jobs USING btree (job_status);

CREATE INDEX ix_jobs_processing_started ON public.jobs USING btree (processing_started);

CREATE INDEX ix_jobs_scheduled_for ON public.jobs USING btree (scheduled_for);

CREATE INDEX ix_jobs_service_id ON public.jobs USING btree (service_id);

CREATE INDEX ix_jobs_template_id ON public.jobs USING btree (template_id);

CREATE INDEX ix_login_events_user_id ON public.login_events USING btree (user_id);

CREATE INDEX ix_monthly_notification_stats_notification_type ON public.monthly_notification_stats_summary USING btree (notification_type);

CREATE INDEX ix_monthly_notification_stats_updated_at ON public.monthly_notification_stats_summary USING btree (updated_at);

CREATE INDEX ix_notification_history_api_key_id ON public.notification_history USING btree (api_key_id);

CREATE INDEX ix_notification_history_api_key_id_created ON public.notification_history USING btree (api_key_id, created_at);

CREATE INDEX ix_notification_history_created_api_key_id ON public.notification_history USING btree (created_at, api_key_id);

CREATE INDEX ix_notification_history_created_at ON public.notification_history USING btree (created_at);

CREATE INDEX ix_notification_history_created_by_id ON public.notification_history USING btree (created_by_id);

CREATE INDEX ix_notification_history_feedback_reason ON public.notification_history USING btree (feedback_reason);

CREATE INDEX ix_notification_history_feedback_type ON public.notification_history USING btree (feedback_type);

CREATE INDEX ix_notification_history_job_id ON public.notification_history USING btree (job_id);

CREATE INDEX ix_notification_history_key_type ON public.notification_history USING btree (key_type);

CREATE INDEX ix_notification_history_notification_status ON public.notification_history USING btree (notification_status);

CREATE INDEX ix_notification_history_notification_type ON public.notification_history USING btree (notification_type);

CREATE INDEX ix_notification_history_reference ON public.notification_history USING btree (reference);

CREATE INDEX ix_notification_history_service_id ON public.notification_history USING btree (service_id);

CREATE INDEX ix_notification_history_service_id_created_at ON public.notification_history USING btree (service_id, date(created_at));

CREATE INDEX ix_notification_history_template_id ON public.notification_history USING btree (template_id);

CREATE INDEX ix_notification_history_week_created ON public.notification_history USING btree (date_trunc('week'::text, created_at));

CREATE INDEX ix_notifications_api_key_id ON public.notifications USING btree (api_key_id);

CREATE INDEX ix_notifications_client_reference ON public.notifications USING btree (client_reference);

CREATE INDEX ix_notifications_created_at ON public.notifications USING btree (created_at);

CREATE INDEX ix_notifications_feedback_reason ON public.notifications USING btree (feedback_reason);

CREATE INDEX ix_notifications_feedback_type ON public.notifications USING btree (feedback_type);

CREATE INDEX ix_notifications_job_id ON public.notifications USING btree (job_id);

CREATE INDEX ix_notifications_key_type ON public.notifications USING btree (key_type);

CREATE INDEX ix_notifications_notification_status ON public.notifications USING btree (notification_status);

CREATE INDEX ix_notifications_notification_type ON public.notifications USING btree (notification_type);

CREATE INDEX ix_notifications_reference ON public.notifications USING btree (reference);

CREATE INDEX ix_notifications_service_created_at ON public.notifications USING btree (service_id, created_at);

CREATE INDEX ix_notifications_service_id ON public.notifications USING btree (service_id);

CREATE INDEX ix_notifications_service_id_created_at ON public.notifications USING btree (service_id, date(created_at));

CREATE INDEX ix_notifications_template_id ON public.notifications USING btree (template_id);

CREATE INDEX ix_permissions_service_id ON public.permissions USING btree (service_id);

CREATE INDEX ix_permissions_user_id ON public.permissions USING btree (user_id);

CREATE INDEX ix_provider_details_created_by_id ON public.provider_details USING btree (created_by_id);

CREATE INDEX ix_provider_details_history_created_by_id ON public.provider_details_history USING btree (created_by_id);

CREATE INDEX ix_provider_rates_provider_id ON public.provider_rates USING btree (provider_id);

CREATE INDEX ix_rates_notification_type ON public.rates USING btree (notification_type);

CREATE INDEX ix_reports_service_id ON public.reports USING btree (service_id);

CREATE INDEX ix_scheduled_notifications_notification_id ON public.scheduled_notifications USING btree (notification_id);

CREATE INDEX ix_service_callback_api_history_service_id ON public.service_callback_api_history USING btree (service_id);

CREATE INDEX ix_service_callback_api_history_updated_by_id ON public.service_callback_api_history USING btree (updated_by_id);

CREATE INDEX ix_service_callback_api_service_id ON public.service_callback_api USING btree (service_id);

CREATE INDEX ix_service_callback_api_updated_by_id ON public.service_callback_api USING btree (updated_by_id);

CREATE INDEX ix_service_data_retention_service_id ON public.service_data_retention USING btree (service_id);

CREATE INDEX ix_service_email_reply_to_service_id ON public.service_email_reply_to USING btree (service_id);

CREATE INDEX ix_service_history_sensitive_service ON public.services_history USING btree (sensitive_service);

CREATE INDEX ix_service_id_notification_type ON public.annual_limits_data USING btree (service_id, notification_type);

CREATE INDEX ix_service_id_notification_type_time ON public.annual_limits_data USING btree (time_period, service_id, notification_type);

CREATE INDEX ix_service_inbound_api_history_service_id ON public.service_inbound_api_history USING btree (service_id);

CREATE INDEX ix_service_inbound_api_history_updated_by_id ON public.service_inbound_api_history USING btree (updated_by_id);

CREATE INDEX ix_service_inbound_api_updated_by_id ON public.service_inbound_api USING btree (updated_by_id);

CREATE INDEX ix_service_letter_contacts_service_id ON public.service_letter_contacts USING btree (service_id);

CREATE INDEX ix_service_permissions_permission ON public.service_permissions USING btree (permission);

CREATE INDEX ix_service_permissions_service_id ON public.service_permissions USING btree (service_id);

CREATE INDEX ix_service_sensitive_service ON public.services USING btree (sensitive_service);

CREATE INDEX ix_service_sms_senders_service_id ON public.service_sms_senders USING btree (service_id);

CREATE INDEX ix_service_whitelist_service_id ON public.service_safelist USING btree (service_id);

CREATE INDEX ix_services_created_by_id ON public.services USING btree (created_by_id);

CREATE INDEX ix_services_history_created_by_id ON public.services_history USING btree (created_by_id);

CREATE INDEX ix_services_history_organisation_id ON public.services_history USING btree (organisation_id);

CREATE INDEX ix_services_organisation_id ON public.services USING btree (organisation_id);

CREATE INDEX ix_template_categories_created_by_id ON public.template_categories USING btree (created_by_id);

CREATE INDEX ix_template_categories_name_en ON public.template_categories USING btree (name_en);

CREATE INDEX ix_template_categories_name_fr ON public.template_categories USING btree (name_fr);

CREATE INDEX ix_template_categories_updated_by_id ON public.template_categories USING btree (updated_by_id);

CREATE INDEX ix_template_category_id ON public.templates USING btree (template_category_id);

CREATE INDEX ix_template_redacted_updated_by_id ON public.template_redacted USING btree (updated_by_id);

CREATE INDEX ix_templates_created_by_id ON public.templates USING btree (created_by_id);

CREATE INDEX ix_templates_history_created_by_id ON public.templates_history USING btree (created_by_id);

CREATE INDEX ix_templates_history_process_type ON public.templates_history USING btree (process_type);

CREATE INDEX ix_templates_history_service_id ON public.templates_history USING btree (service_id);

CREATE INDEX ix_templates_process_type ON public.templates USING btree (process_type);

CREATE INDEX ix_templates_service_id ON public.templates USING btree (service_id);

CREATE INDEX ix_users_auth_type ON public.users USING btree (auth_type);

CREATE INDEX ix_users_name ON public.users USING btree (name);

CREATE INDEX ix_verify_codes_user_id ON public.verify_codes USING btree (user_id);

ALTER TABLE ONLY public.annual_billing
    ADD CONSTRAINT annual_billing_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_key_type_fkey FOREIGN KEY (key_type) REFERENCES public.key_types(name);

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.complaints
    ADD CONSTRAINT complaints_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.domain
    ADD CONSTRAINT domain_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_brand_type_fkey FOREIGN KEY (brand_type) REFERENCES public.branding_type(name);

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.fido2_keys
    ADD CONSTRAINT fido2_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.fido2_sessions
    ADD CONSTRAINT fido2_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT fk_api_keys_created_by_id FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT fk_email_branding_organisation FOREIGN KEY (organisation_id) REFERENCES public.organisation(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT fk_notification_history_notification_status FOREIGN KEY (notification_status) REFERENCES public.notification_status_types(name);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT fk_notifications_notification_status FOREIGN KEY (notification_status) REFERENCES public.notification_status_types(name);

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT fk_organisation_agreement_user_id FOREIGN KEY (agreement_signed_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT fk_organisation_letter_branding_id FOREIGN KEY (letter_branding_id) REFERENCES public.letter_branding(id);

ALTER TABLE ONLY public.annual_limits_data
    ADD CONSTRAINT fk_service_id FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT fk_service_organisation FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT fk_services_created_by_id FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT fk_services_go_live_user FOREIGN KEY (go_live_user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT fk_template_template_categories FOREIGN KEY (template_category_id) REFERENCES public.template_categories(id);

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT fk_templates_created_by_id FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.inbound_numbers
    ADD CONSTRAINT inbound_numbers_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.inbound_sms
    ADD CONSTRAINT inbound_sms_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_invited_by_id_fkey FOREIGN KEY (invited_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_status_fkey FOREIGN KEY (status) REFERENCES public.invite_status_type(name);

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_auth_type_fkey FOREIGN KEY (auth_type) REFERENCES public.auth_type(name);

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_api_keys_id_fkey FOREIGN KEY (api_key_id) REFERENCES public.api_keys(id);

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_job_status_fkey FOREIGN KEY (job_status) REFERENCES public.job_status(name);

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.templates(id);

ALTER TABLE ONLY public.login_events
    ADD CONSTRAINT login_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_api_key_id_fkey FOREIGN KEY (api_key_id) REFERENCES public.api_keys(id);

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id);

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_key_type_fkey FOREIGN KEY (key_type) REFERENCES public.key_types(name);

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_templates_history_fkey FOREIGN KEY (template_id, template_version) REFERENCES public.templates_history(id, version);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_api_key_id_fkey FOREIGN KEY (api_key_id) REFERENCES public.api_keys(id);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_key_type_fkey FOREIGN KEY (key_type) REFERENCES public.key_types(name);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_templates_history_fkey FOREIGN KEY (template_id, template_version) REFERENCES public.templates_history(id, version);

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT organisation_organisation_type_fkey FOREIGN KEY (organisation_type) REFERENCES public.organisation_types(name);

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.provider_details
    ADD CONSTRAINT provider_details_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.provider_details_history
    ADD CONSTRAINT provider_details_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.provider_rates
    ADD CONSTRAINT provider_rate_to_provider_fk FOREIGN KEY (provider_id) REFERENCES public.provider_details(id);

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id);

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_requesting_user_id_fkey FOREIGN KEY (requesting_user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.scheduled_notifications
    ADD CONSTRAINT scheduled_notifications_notification_id_fkey FOREIGN KEY (notification_id) REFERENCES public.notifications(id);

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_type_fk FOREIGN KEY (callback_type) REFERENCES public.service_callback_type(name);

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.service_data_retention
    ADD CONSTRAINT service_data_retention_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_email_branding
    ADD CONSTRAINT service_email_branding_email_branding_id_fkey FOREIGN KEY (email_branding_id) REFERENCES public.email_branding(id);

ALTER TABLE ONLY public.service_email_branding
    ADD CONSTRAINT service_email_branding_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_email_reply_to
    ADD CONSTRAINT service_email_reply_to_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_inbound_api
    ADD CONSTRAINT service_inbound_api_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_inbound_api
    ADD CONSTRAINT service_inbound_api_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.service_letter_branding
    ADD CONSTRAINT service_letter_branding_letter_branding_id_fkey FOREIGN KEY (letter_branding_id) REFERENCES public.letter_branding(id);

ALTER TABLE ONLY public.service_letter_branding
    ADD CONSTRAINT service_letter_branding_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_letter_contacts
    ADD CONSTRAINT service_letter_contacts_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_permissions
    ADD CONSTRAINT service_permissions_permission_fkey FOREIGN KEY (permission) REFERENCES public.service_permission_types(name);

ALTER TABLE ONLY public.service_permissions
    ADD CONSTRAINT service_permissions_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_sms_senders
    ADD CONSTRAINT service_sms_senders_inbound_number_id_fkey FOREIGN KEY (inbound_number_id) REFERENCES public.inbound_numbers(id);

ALTER TABLE ONLY public.service_sms_senders
    ADD CONSTRAINT service_sms_senders_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.service_safelist
    ADD CONSTRAINT service_whitelist_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.services_history
    ADD CONSTRAINT services_history_suspended_by_id_fkey FOREIGN KEY (suspended_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_organisation_type_fkey FOREIGN KEY (organisation_type) REFERENCES public.organisation_types(name);

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_suspended_by_id_fkey FOREIGN KEY (suspended_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.template_folder_map
    ADD CONSTRAINT template_folder_map_template_folder_id_fkey FOREIGN KEY (template_folder_id) REFERENCES public.template_folder(id);

ALTER TABLE ONLY public.template_folder_map
    ADD CONSTRAINT template_folder_map_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.templates(id);

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT template_folder_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.template_folder(id);

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT template_folder_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.template_redacted
    ADD CONSTRAINT template_redacted_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.templates(id);

ALTER TABLE ONLY public.template_redacted
    ADD CONSTRAINT template_redacted_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_process_type_fkey FOREIGN KEY (process_type) REFERENCES public.template_process_type(name);

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_service_letter_contact_id_fkey FOREIGN KEY (service_letter_contact_id) REFERENCES public.service_letter_contacts(id);

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_process_type_fkey FOREIGN KEY (process_type) REFERENCES public.template_process_type(name);

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_service_letter_contact_id_fkey FOREIGN KEY (service_letter_contact_id) REFERENCES public.service_letter_contacts(id);

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_template_folder_id_fkey FOREIGN KEY (template_folder_id) REFERENCES public.template_folder(id);

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_template_folder_id_service_id_fkey FOREIGN KEY (template_folder_id, service_id) REFERENCES public.template_folder(id, service_id);

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_user_id_service_id_fkey FOREIGN KEY (user_id, service_id) REFERENCES public.user_to_service(user_id, service_id);

ALTER TABLE ONLY public.user_to_organisation
    ADD CONSTRAINT user_to_organisation_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);

ALTER TABLE ONLY public.user_to_organisation
    ADD CONSTRAINT user_to_organisation_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.user_to_service
    ADD CONSTRAINT user_to_service_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);

ALTER TABLE ONLY public.user_to_service
    ADD CONSTRAINT user_to_service_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_auth_type_fkey FOREIGN KEY (auth_type) REFERENCES public.auth_type(name);

ALTER TABLE ONLY public.verify_codes
    ADD CONSTRAINT verify_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);
