--
-- PostgreSQL database dump
--

\restrict eR9wjtmhT4bdsi7OfU5qb8707ivNySeaj09oFXGEA2LdhuPPDYWY9DPhQOljn6z

-- Dumped from database version 16.11 (Debian 16.11-1.pgdg12+1)
-- Dumped by pg_dump version 16.11 (Debian 16.11-1.pgdg12+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: invited_users_status_types; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.invited_users_status_types AS ENUM (
    'pending',
    'accepted',
    'cancelled'
);


ALTER TYPE public.invited_users_status_types OWNER TO postgres;

--
-- Name: job_status_types; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.job_status_types AS ENUM (
    'pending',
    'in progress',
    'finished',
    'sending limits exceeded'
);


ALTER TYPE public.job_status_types OWNER TO postgres;

--
-- Name: notification_feedback_subtypes; Type: TYPE; Schema: public; Owner: postgres
--

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


ALTER TYPE public.notification_feedback_subtypes OWNER TO postgres;

--
-- Name: notification_feedback_types; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.notification_feedback_types AS ENUM (
    'hard-bounce',
    'soft-bounce',
    'unknown-bounce'
);


ALTER TYPE public.notification_feedback_types OWNER TO postgres;

--
-- Name: notification_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.notification_type AS ENUM (
    'email',
    'sms',
    'letter'
);


ALTER TYPE public.notification_type OWNER TO postgres;

--
-- Name: notify_status_type; Type: TYPE; Schema: public; Owner: postgres
--

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


ALTER TYPE public.notify_status_type OWNER TO postgres;

--
-- Name: permission_types; Type: TYPE; Schema: public; Owner: postgres
--

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


ALTER TYPE public.permission_types OWNER TO postgres;

--
-- Name: recipient_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.recipient_type AS ENUM (
    'mobile',
    'email'
);


ALTER TYPE public.recipient_type OWNER TO postgres;

--
-- Name: sms_sending_vehicle; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.sms_sending_vehicle AS ENUM (
    'short_code',
    'long_code'
);


ALTER TYPE public.sms_sending_vehicle OWNER TO postgres;

--
-- Name: template_type; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.template_type AS ENUM (
    'sms',
    'email',
    'letter'
);


ALTER TYPE public.template_type OWNER TO postgres;

--
-- Name: verify_code_types; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.verify_code_types AS ENUM (
    'email',
    'sms'
);


ALTER TYPE public.verify_code_types OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: alembic_version; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.alembic_version (
    version_num character varying(32) NOT NULL
);


ALTER TABLE public.alembic_version OWNER TO postgres;

--
-- Name: annual_billing; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.annual_billing (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    financial_year_start integer NOT NULL,
    free_sms_fragment_limit integer NOT NULL,
    updated_at timestamp without time zone,
    created_at timestamp without time zone NOT NULL
);


ALTER TABLE public.annual_billing OWNER TO postgres;

--
-- Name: annual_limits_data; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.annual_limits_data (
    service_id uuid NOT NULL,
    time_period character varying NOT NULL,
    annual_email_limit bigint NOT NULL,
    annual_sms_limit bigint NOT NULL,
    notification_type character varying NOT NULL,
    notification_count bigint NOT NULL
);


ALTER TABLE public.annual_limits_data OWNER TO postgres;

--
-- Name: api_keys; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: api_keys_history; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: auth_type; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.auth_type (
    name character varying NOT NULL
);


ALTER TABLE public.auth_type OWNER TO postgres;

--
-- Name: branding_type; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.branding_type (
    name character varying(255) NOT NULL
);


ALTER TABLE public.branding_type OWNER TO postgres;

--
-- Name: complaints; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: daily_sorted_letter; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.daily_sorted_letter (
    id uuid NOT NULL,
    billing_day date NOT NULL,
    unsorted_count integer NOT NULL,
    sorted_count integer NOT NULL,
    updated_at timestamp without time zone,
    file_name character varying
);


ALTER TABLE public.daily_sorted_letter OWNER TO postgres;

--
-- Name: dm_datetime; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: domain; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.domain (
    domain character varying(255) NOT NULL,
    organisation_id uuid NOT NULL
);


ALTER TABLE public.domain OWNER TO postgres;

--
-- Name: email_branding; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: events; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.events (
    id uuid NOT NULL,
    event_type character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL,
    data json NOT NULL
);


ALTER TABLE public.events OWNER TO postgres;

--
-- Name: fido2_keys; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.fido2_keys (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    name character varying NOT NULL,
    key text NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone
);


ALTER TABLE public.fido2_keys OWNER TO postgres;

--
-- Name: fido2_sessions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.fido2_sessions (
    user_id uuid NOT NULL,
    session text NOT NULL,
    created_at timestamp without time zone NOT NULL
);


ALTER TABLE public.fido2_sessions OWNER TO postgres;

--
-- Name: ft_billing; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: ft_notification_status; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: inbound_numbers; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: inbound_sms; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: invite_status_type; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.invite_status_type (
    name character varying NOT NULL
);


ALTER TABLE public.invite_status_type OWNER TO postgres;

--
-- Name: invited_organisation_users; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.invited_organisation_users (
    id uuid NOT NULL,
    email_address character varying(255) NOT NULL,
    invited_by_id uuid NOT NULL,
    organisation_id uuid NOT NULL,
    created_at timestamp without time zone NOT NULL,
    status character varying NOT NULL
);


ALTER TABLE public.invited_organisation_users OWNER TO postgres;

--
-- Name: invited_users; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: job_status; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.job_status (
    name character varying(255) NOT NULL
);


ALTER TABLE public.job_status OWNER TO postgres;

--
-- Name: jobs; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN jobs.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.id IS 'Unique identifier for each job. Primary key for the table.';


--
-- Name: COLUMN jobs.original_file_name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.original_file_name IS 'Name of the file that was uploaded to create this job.';


--
-- Name: COLUMN jobs.service_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.service_id IS 'Foreign key linking to the service running the job.';


--
-- Name: COLUMN jobs.template_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.template_id IS 'Foreign key referencing the template used for notifications in this job.';


--
-- Name: COLUMN jobs.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.created_at IS 'Date and time when the job was created.';


--
-- Name: COLUMN jobs.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.updated_at IS 'Date and time when the job was last updated.';


--
-- Name: COLUMN jobs.notification_count; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.notification_count IS 'Total number of notifications to be sent in this job.';


--
-- Name: COLUMN jobs.notifications_sent; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.notifications_sent IS 'Number of notifications that have been sent.';


--
-- Name: COLUMN jobs.processing_started; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.processing_started IS 'Date and time when the job started being processed.';


--
-- Name: COLUMN jobs.processing_finished; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.processing_finished IS 'Date and time when the job finished processing.';


--
-- Name: COLUMN jobs.created_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.created_by_id IS 'Foreign key referencing the user who created this job.';


--
-- Name: COLUMN jobs.template_version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.template_version IS 'Version number of the template used for this job.';


--
-- Name: COLUMN jobs.notifications_delivered; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.notifications_delivered IS 'Number of notifications successfully delivered to recipients.';


--
-- Name: COLUMN jobs.notifications_failed; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.notifications_failed IS 'Number of notifications that failed to deliver.';


--
-- Name: COLUMN jobs.job_status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.job_status IS 'Status of the job.';


--
-- Name: COLUMN jobs.scheduled_for; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.scheduled_for IS 'Date and time when the job is scheduled to run, for delayed notifications.';


--
-- Name: COLUMN jobs.archived; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.archived IS 'Flag indicating whether the job has been archived.';


--
-- Name: COLUMN jobs.api_key_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.api_key_id IS 'Foreign key referencing the API key used to create this job.';


--
-- Name: COLUMN jobs.sender_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.jobs.sender_id IS 'Sender identity used for this job.';


--
-- Name: key_types; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.key_types (
    name character varying(255) NOT NULL
);


ALTER TABLE public.key_types OWNER TO postgres;

--
-- Name: letter_branding; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.letter_branding (
    id uuid NOT NULL,
    name character varying(255) NOT NULL,
    filename character varying(255) NOT NULL
);


ALTER TABLE public.letter_branding OWNER TO postgres;

--
-- Name: letter_rates; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: login_events; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.login_events (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    data jsonb NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone
);


ALTER TABLE public.login_events OWNER TO postgres;

--
-- Name: COLUMN login_events.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.login_events.id IS 'Unique identifier for each login event';


--
-- Name: COLUMN login_events.user_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.login_events.user_id IS 'Foreign key referencing the user that logged in..';


--
-- Name: COLUMN login_events.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.login_events.created_at IS 'Date and time when the login event was created.';


--
-- Name: COLUMN login_events.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.login_events.updated_at IS 'Date and time when the login event was updated.';


--
-- Name: monthly_notification_stats_summary; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.monthly_notification_stats_summary (
    month text NOT NULL,
    service_id uuid NOT NULL,
    notification_type text NOT NULL,
    notification_count integer NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.monthly_notification_stats_summary OWNER TO postgres;

--
-- Name: notification_history; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN notification_history.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.id IS 'Unique identifier for each notification record.';


--
-- Name: COLUMN notification_history.job_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.job_id IS 'Foreign key reference to the batch job that generated this notification.';


--
-- Name: COLUMN notification_history.job_row_number; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.job_row_number IS 'Sequential number indicating the position of this notification within its parent job.';


--
-- Name: COLUMN notification_history.service_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.service_id IS 'Foreign key reference to the service that sent the notification.';


--
-- Name: COLUMN notification_history.template_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.template_id IS 'Foreign key reference to the message template used for this notification.';


--
-- Name: COLUMN notification_history.template_version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.template_version IS 'Version number of the template used.';


--
-- Name: COLUMN notification_history.api_key_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.api_key_id IS 'Foreign key reference to the API key used to authenticate the notification request.';


--
-- Name: COLUMN notification_history.key_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.key_type IS 'Type of API key used.';


--
-- Name: COLUMN notification_history.notification_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.notification_type IS 'Categorization of notification (email or sms).';


--
-- Name: COLUMN notification_history.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.created_at IS 'Date and time when the notification record was created.';


--
-- Name: COLUMN notification_history.sent_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.sent_at IS 'Date and time when the notification was sent.';


--
-- Name: COLUMN notification_history.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.updated_at IS 'Date and time when the notification record was updated.';


--
-- Name: COLUMN notification_history.reference; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.reference IS 'AWS provided message ID of the notification.';


--
-- Name: COLUMN notification_history.billable_units; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.billable_units IS 'Number of billing units consumed by this notification. For SMS this is the number of message fragments.';


--
-- Name: COLUMN notification_history.client_reference; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.client_reference IS 'Customer-provided reference for their own tracking purposes.';


--
-- Name: COLUMN notification_history.international; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.international IS 'Flag indicating whether the notification was sent internationally.';


--
-- Name: COLUMN notification_history.phone_prefix; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.phone_prefix IS 'Country code prefix for phone numbers in SMS notifications.';


--
-- Name: COLUMN notification_history.rate_multiplier; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.rate_multiplier IS 'Pricing multiplier applied to SMS notifications based on the billable_units.';


--
-- Name: COLUMN notification_history.notification_status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.notification_status IS 'Status of the notification.';


--
-- Name: COLUMN notification_history.created_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.created_by_id IS 'Identifier for the user that initiated the notification.';


--
-- Name: COLUMN notification_history.queue_name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.queue_name IS 'Name of the processing queue this notification was handled by.';


--
-- Name: COLUMN notification_history.feedback_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.feedback_type IS 'Category of delivery feedback received.';


--
-- Name: COLUMN notification_history.feedback_subtype; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.feedback_subtype IS 'More specific feedback classification.';


--
-- Name: COLUMN notification_history.ses_feedback_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.ses_feedback_id IS 'Amazon SES specific feedback identifier for email notifications.';


--
-- Name: COLUMN notification_history.ses_feedback_date; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.ses_feedback_date IS 'Date and time when feedback was received from Amazon SES.';


--
-- Name: COLUMN notification_history.sms_total_message_price; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.sms_total_message_price IS 'Total cost charged for the SMS message.';


--
-- Name: COLUMN notification_history.sms_total_carrier_fee; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.sms_total_carrier_fee IS 'Portion of the total cost that goes to the carrier.';


--
-- Name: COLUMN notification_history.sms_iso_country_code; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.sms_iso_country_code IS 'ISO country code for the destination of the SMS.';


--
-- Name: COLUMN notification_history.sms_carrier_name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.sms_carrier_name IS 'Name of the carrier that delivered the SMS message.';


--
-- Name: COLUMN notification_history.sms_message_encoding; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.sms_message_encoding IS 'Character encoding used for the SMS message.';


--
-- Name: COLUMN notification_history.sms_origination_phone_number; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.sms_origination_phone_number IS 'Phone number that sent the SMS message.';


--
-- Name: COLUMN notification_history.feedback_reason; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notification_history.feedback_reason IS 'Pinpoint failure reason when an SMS message cannot be delivered.';


--
-- Name: notification_status_types; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.notification_status_types (
    name character varying NOT NULL
);


ALTER TABLE public.notification_status_types OWNER TO postgres;

--
-- Name: notifications; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN notifications.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.id IS 'Unique identifier for each notification record.';


--
-- Name: COLUMN notifications.job_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.job_id IS 'Foreign key reference to the batch job that generated this notification.';


--
-- Name: COLUMN notifications.service_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.service_id IS 'Foreign key reference to the service that sent the notification.';


--
-- Name: COLUMN notifications.template_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.template_id IS 'Foreign key reference to the message template used for this notification.';


--
-- Name: COLUMN notifications.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.created_at IS 'Date and time when the notification record was created.';


--
-- Name: COLUMN notifications.sent_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.sent_at IS 'Date and time when the notification was sent.';


--
-- Name: COLUMN notifications.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.updated_at IS 'Date and time when the notification record was updated.';


--
-- Name: COLUMN notifications.reference; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.reference IS 'AWS provided message ID of the notification.';


--
-- Name: COLUMN notifications.template_version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.template_version IS 'Version number of the template used.';


--
-- Name: COLUMN notifications.job_row_number; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.job_row_number IS 'Sequential number indicating the position of this notification within its parent job.';


--
-- Name: COLUMN notifications.api_key_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.api_key_id IS 'Foreign key reference to the API key used to authenticate the notification request.';


--
-- Name: COLUMN notifications.key_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.key_type IS 'Type of API key used.';


--
-- Name: COLUMN notifications.notification_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.notification_type IS 'Categorization of notification (email or sms).';


--
-- Name: COLUMN notifications.billable_units; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.billable_units IS 'Number of billing units consumed by this notification. For SMS this is the number of message fragments.';


--
-- Name: COLUMN notifications.client_reference; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.client_reference IS 'Customer-provided reference for their own tracking purposes.';


--
-- Name: COLUMN notifications.international; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.international IS 'Flag indicating whether the notification was sent internationally.';


--
-- Name: COLUMN notifications.phone_prefix; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.phone_prefix IS 'Country code prefix for phone numbers in SMS notifications.';


--
-- Name: COLUMN notifications.rate_multiplier; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.rate_multiplier IS 'Pricing multiplier applied to SMS notifications based on the billable_units.';


--
-- Name: COLUMN notifications.notification_status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.notification_status IS 'Status of the notification.';


--
-- Name: COLUMN notifications.created_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.created_by_id IS 'Identifier for the user that initiated the notification.';


--
-- Name: COLUMN notifications.queue_name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.queue_name IS 'Name of the processing queue this notification was handled by.';


--
-- Name: COLUMN notifications.feedback_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.feedback_type IS 'Category of delivery feedback received.';


--
-- Name: COLUMN notifications.feedback_subtype; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.feedback_subtype IS 'More specific feedback classification.';


--
-- Name: COLUMN notifications.ses_feedback_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.ses_feedback_id IS 'Amazon SES specific feedback identifier for email notifications.';


--
-- Name: COLUMN notifications.ses_feedback_date; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.ses_feedback_date IS 'Date and time when feedback was received from Amazon SES.';


--
-- Name: COLUMN notifications.sms_total_message_price; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.sms_total_message_price IS 'Total cost charged for the SMS message.';


--
-- Name: COLUMN notifications.sms_total_carrier_fee; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.sms_total_carrier_fee IS 'Portion of the total cost that goes to the carrier.';


--
-- Name: COLUMN notifications.sms_iso_country_code; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.sms_iso_country_code IS 'ISO country code for the destination of the SMS.';


--
-- Name: COLUMN notifications.sms_carrier_name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.sms_carrier_name IS 'Name of the carrier that delivered the SMS message.';


--
-- Name: COLUMN notifications.sms_message_encoding; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.sms_message_encoding IS 'Character encoding used for the SMS message.';


--
-- Name: COLUMN notifications.sms_origination_phone_number; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.sms_origination_phone_number IS 'Phone number that sent the SMS message.';


--
-- Name: COLUMN notifications.feedback_reason; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.notifications.feedback_reason IS 'Pinpoint failure reason when an SMS message cannot be delivered.';


--
-- Name: organisation; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN organisation.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.id IS 'Unique identifier for the organisation.';


--
-- Name: COLUMN organisation.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.name IS 'Name of the organisation.';


--
-- Name: COLUMN organisation.active; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.active IS 'Indicates whether the organisation is currently active in the system.';


--
-- Name: COLUMN organisation.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.created_at IS 'Date and time when the organisation record was created.';


--
-- Name: COLUMN organisation.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.updated_at IS 'Date and time when the organisation record was last updated.';


--
-- Name: COLUMN organisation.email_branding_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.email_branding_id IS 'Email branding configuration used by this organisation.';


--
-- Name: COLUMN organisation.agreement_signed; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.agreement_signed IS 'Indicates whether the organisation has signed the service agreement.';


--
-- Name: COLUMN organisation.agreement_signed_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.agreement_signed_at IS 'Date and time when the service agreement was signed.';


--
-- Name: COLUMN organisation.agreement_signed_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.agreement_signed_by_id IS 'Foreign key reference to the user who signed the service agreement.';


--
-- Name: COLUMN organisation.agreement_signed_version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.agreement_signed_version IS 'Version number of the service agreement that was signed';


--
-- Name: COLUMN organisation.crown; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.crown IS 'Indicates whether this is a Crown Corporation.';


--
-- Name: COLUMN organisation.organisation_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.organisation_type IS 'Type of the organisation.';


--
-- Name: COLUMN organisation.default_branding_is_french; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.organisation.default_branding_is_french IS 'Indicates whether the default branding for this organisation';


--
-- Name: organisation_types; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.organisation_types (
    name character varying(255) NOT NULL,
    is_crown boolean,
    annual_free_sms_fragment_limit bigint NOT NULL
);


ALTER TABLE public.organisation_types OWNER TO postgres;

--
-- Name: permissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.permissions (
    id uuid NOT NULL,
    service_id uuid,
    user_id uuid NOT NULL,
    permission public.permission_types NOT NULL,
    created_at timestamp without time zone NOT NULL
);


ALTER TABLE public.permissions OWNER TO postgres;

--
-- Name: COLUMN permissions.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.permissions.id IS 'Unique identifier for the permission.';


--
-- Name: COLUMN permissions.service_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.permissions.service_id IS 'Foreign key reference to the service this permission applies to.';


--
-- Name: COLUMN permissions.user_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.permissions.user_id IS 'Foreign key reference to the user the permission has been granted to.';


--
-- Name: COLUMN permissions.permission; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.permissions.permission IS 'The permission that has been granted.';


--
-- Name: COLUMN permissions.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.permissions.created_at IS 'Date and time when the permission was assigned.';


--
-- Name: provider_details; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: provider_details_history; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: provider_rates; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.provider_rates (
    id uuid NOT NULL,
    valid_from timestamp without time zone NOT NULL,
    rate numeric NOT NULL,
    provider_id uuid NOT NULL
);


ALTER TABLE public.provider_rates OWNER TO postgres;

--
-- Name: rates; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.rates (
    id uuid NOT NULL,
    valid_from timestamp without time zone NOT NULL,
    rate numeric NOT NULL,
    notification_type public.notification_type NOT NULL,
    sms_sending_vehicle public.sms_sending_vehicle DEFAULT 'long_code'::public.sms_sending_vehicle NOT NULL
);


ALTER TABLE public.rates OWNER TO postgres;

--
-- Name: reports; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: scheduled_notifications; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.scheduled_notifications (
    id uuid NOT NULL,
    notification_id uuid NOT NULL,
    scheduled_for timestamp without time zone NOT NULL,
    pending boolean NOT NULL
);


ALTER TABLE public.scheduled_notifications OWNER TO postgres;

--
-- Name: service_callback_api; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: service_callback_api_history; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: service_callback_type; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.service_callback_type (
    name character varying NOT NULL
);


ALTER TABLE public.service_callback_type OWNER TO postgres;

--
-- Name: service_data_retention; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.service_data_retention (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    notification_type public.notification_type NOT NULL,
    days_of_retention integer NOT NULL,
    created_at timestamp without time zone NOT NULL,
    updated_at timestamp without time zone
);


ALTER TABLE public.service_data_retention OWNER TO postgres;

--
-- Name: service_email_branding; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.service_email_branding (
    service_id uuid NOT NULL,
    email_branding_id uuid NOT NULL
);


ALTER TABLE public.service_email_branding OWNER TO postgres;

--
-- Name: service_email_reply_to; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: service_inbound_api; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: service_inbound_api_history; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: service_letter_branding; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.service_letter_branding (
    service_id uuid NOT NULL,
    letter_branding_id uuid NOT NULL
);


ALTER TABLE public.service_letter_branding OWNER TO postgres;

--
-- Name: service_letter_contacts; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: service_permission_types; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.service_permission_types (
    name character varying(255) NOT NULL
);


ALTER TABLE public.service_permission_types OWNER TO postgres;

--
-- Name: service_permissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.service_permissions (
    service_id uuid NOT NULL,
    permission character varying(255) NOT NULL,
    created_at timestamp without time zone NOT NULL
);


ALTER TABLE public.service_permissions OWNER TO postgres;

--
-- Name: service_safelist; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.service_safelist (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    recipient_type public.recipient_type NOT NULL,
    recipient character varying(255) NOT NULL,
    created_at timestamp without time zone
);


ALTER TABLE public.service_safelist OWNER TO postgres;

--
-- Name: service_sms_senders; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: services; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN services.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.id IS 'Unique identifier for the service.';


--
-- Name: COLUMN services.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.name IS 'Name of the service.';


--
-- Name: COLUMN services.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.created_at IS 'Date and time when the service was created.';


--
-- Name: COLUMN services.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.updated_at IS 'Date and time when the service was last updated.';


--
-- Name: COLUMN services.active; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.active IS 'Indicates whether the service is currently active.';


--
-- Name: COLUMN services.message_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.message_limit IS 'Maximum number of messages this service can send.';


--
-- Name: COLUMN services.restricted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.restricted IS 'Indicates if this is a trial service.';


--
-- Name: COLUMN services.email_from; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.email_from IS 'Email address that appears in the ''From'' field when sending email notifications.';


--
-- Name: COLUMN services.created_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.created_by_id IS 'Identifier of the user who created this service.';


--
-- Name: COLUMN services.version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.version IS 'Current version of the service configuration.';


--
-- Name: COLUMN services.research_mode; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.research_mode IS 'Indicates if this service is in research mode.';


--
-- Name: COLUMN services.organisation_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.organisation_type IS 'Type of organisation this service is classified as.';


--
-- Name: COLUMN services.prefix_sms; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.prefix_sms IS 'Whether SMS messages should include a prefix identifying the service.';


--
-- Name: COLUMN services.crown; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.crown IS 'Indicates if this service is operated by a Crown Corporation.';


--
-- Name: COLUMN services.rate_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.rate_limit IS 'Maximum number of notifications the service can send per time unit.';


--
-- Name: COLUMN services.consent_to_research; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.consent_to_research IS 'Whether the service has consented to being included in research studies.';


--
-- Name: COLUMN services.volume_email; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.volume_email IS 'Anticipated volume of email notifications for this service.';


--
-- Name: COLUMN services.volume_sms; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.volume_sms IS 'Anticipated volume of SMS notifications for this service.';


--
-- Name: COLUMN services.count_as_live; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.count_as_live IS 'Whether this service should be counted as a live production service.';


--
-- Name: COLUMN services.go_live_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.go_live_at IS 'Date and time when this service went live.';


--
-- Name: COLUMN services.go_live_user_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.go_live_user_id IS 'Identifier of the user who requested the service to go live.';


--
-- Name: COLUMN services.organisation_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.organisation_id IS 'Reference to the organisation that this service belongs to.';


--
-- Name: COLUMN services.sending_domain; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.sending_domain IS 'Domain name used for sending email notifications.';


--
-- Name: COLUMN services.default_branding_is_french; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.default_branding_is_french IS 'Whether the default branding for notifications is in French.';


--
-- Name: COLUMN services.sms_daily_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.sms_daily_limit IS 'Maximum number of SMS messages this service can send per day.';


--
-- Name: COLUMN services.organisation_notes; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.organisation_notes IS 'Additional notes about the organisation in relation to this service.';


--
-- Name: COLUMN services.sensitive_service; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.sensitive_service IS 'Indicates if this service handles sensitive or protected information.';


--
-- Name: COLUMN services.email_annual_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.email_annual_limit IS 'Maximum number of email notifications this service can send annually.';


--
-- Name: COLUMN services.sms_annual_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services.sms_annual_limit IS 'Maximum number of SMS notifications this service can send annually.';


--
-- Name: services_history; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN services_history.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.id IS 'Unique identifier for the service.';


--
-- Name: COLUMN services_history.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.name IS 'Name of the service.';


--
-- Name: COLUMN services_history.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.created_at IS 'Date and time when the service was created.';


--
-- Name: COLUMN services_history.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.updated_at IS 'Date and time when the service was last updated.';


--
-- Name: COLUMN services_history.active; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.active IS 'Indicates whether the service is currently active.';


--
-- Name: COLUMN services_history.message_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.message_limit IS 'Maximum number of messages this service can send.';


--
-- Name: COLUMN services_history.restricted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.restricted IS 'Indicates if this is a trial service.';


--
-- Name: COLUMN services_history.email_from; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.email_from IS 'Email address that appears in the ''From'' field when sending email notifications.';


--
-- Name: COLUMN services_history.created_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.created_by_id IS 'Identifier of the user who created this service.';


--
-- Name: COLUMN services_history.version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.version IS 'Current version of the service configuration.';


--
-- Name: COLUMN services_history.research_mode; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.research_mode IS 'Indicates if this service is in research mode.';


--
-- Name: COLUMN services_history.organisation_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.organisation_type IS 'Type of organisation this service is classified as.';


--
-- Name: COLUMN services_history.prefix_sms; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.prefix_sms IS 'Whether SMS messages should include a prefix identifying the service.';


--
-- Name: COLUMN services_history.crown; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.crown IS 'Indicates if this service is operated by a Crown Corporation.';


--
-- Name: COLUMN services_history.rate_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.rate_limit IS 'Maximum number of notifications the service can send per time unit.';


--
-- Name: COLUMN services_history.consent_to_research; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.consent_to_research IS 'Whether the service has consented to being included in research studies.';


--
-- Name: COLUMN services_history.volume_email; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.volume_email IS 'Anticipated volume of email notifications for this service.';


--
-- Name: COLUMN services_history.volume_sms; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.volume_sms IS 'Anticipated volume of SMS notifications for this service.';


--
-- Name: COLUMN services_history.count_as_live; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.count_as_live IS 'Whether this service should be counted as a live production service.';


--
-- Name: COLUMN services_history.go_live_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.go_live_at IS 'Date and time when this service went live.';


--
-- Name: COLUMN services_history.go_live_user_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.go_live_user_id IS 'Identifier of the user who requested the service to go live.';


--
-- Name: COLUMN services_history.organisation_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.organisation_id IS 'Reference to the organisation that this service belongs to.';


--
-- Name: COLUMN services_history.sending_domain; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.sending_domain IS 'Domain name used for sending email notifications.';


--
-- Name: COLUMN services_history.default_branding_is_french; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.default_branding_is_french IS 'Whether the default branding for notifications is in French.';


--
-- Name: COLUMN services_history.sms_daily_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.sms_daily_limit IS 'Maximum number of SMS messages this service can send per day.';


--
-- Name: COLUMN services_history.organisation_notes; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.organisation_notes IS 'Additional notes about the organisation in relation to this service.';


--
-- Name: COLUMN services_history.sensitive_service; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.sensitive_service IS 'Indicates if this service handles sensitive or protected information.';


--
-- Name: COLUMN services_history.email_annual_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.email_annual_limit IS 'Maximum number of email notifications this service can send annually.';


--
-- Name: COLUMN services_history.sms_annual_limit; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.services_history.sms_annual_limit IS 'Maximum number of SMS notifications this service can send annually.';


--
-- Name: template_categories; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN template_categories.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.id IS 'Unique identifier for the template category.';


--
-- Name: COLUMN template_categories.name_en; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.name_en IS 'English name of the template category.';


--
-- Name: COLUMN template_categories.name_fr; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.name_fr IS 'French name of the template category.';


--
-- Name: COLUMN template_categories.description_en; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.description_en IS 'English description of what this template category is used for.';


--
-- Name: COLUMN template_categories.description_fr; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.description_fr IS 'French description of what this template category is used for.';


--
-- Name: COLUMN template_categories.sms_process_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.sms_process_type IS 'Defines the processing priority of SMS templates in this category.';


--
-- Name: COLUMN template_categories.email_process_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.email_process_type IS 'Defines the processing priority of email templates in this category.';


--
-- Name: COLUMN template_categories.hidden; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.hidden IS 'Indicates whether this category should be hidden in the interface.';


--
-- Name: COLUMN template_categories.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.created_at IS 'Date and time when this template category was created.';


--
-- Name: COLUMN template_categories.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.updated_at IS 'Date and time when this template category was last updated.';


--
-- Name: COLUMN template_categories.sms_sending_vehicle; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.template_categories.sms_sending_vehicle IS 'Defines if templates in this category use a short or long code for sending.';


--
-- Name: template_folder; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.template_folder (
    id uuid NOT NULL,
    service_id uuid NOT NULL,
    name character varying NOT NULL,
    parent_id uuid
);


ALTER TABLE public.template_folder OWNER TO postgres;

--
-- Name: template_folder_map; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.template_folder_map (
    template_id uuid NOT NULL,
    template_folder_id uuid NOT NULL
);


ALTER TABLE public.template_folder_map OWNER TO postgres;

--
-- Name: template_process_type; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.template_process_type (
    name character varying(255) NOT NULL
);


ALTER TABLE public.template_process_type OWNER TO postgres;

--
-- Name: template_redacted; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.template_redacted (
    template_id uuid NOT NULL,
    redact_personalisation boolean NOT NULL,
    updated_at timestamp without time zone NOT NULL,
    updated_by_id uuid NOT NULL
);


ALTER TABLE public.template_redacted OWNER TO postgres;

--
-- Name: templates; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN templates.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.id IS 'Unique identifier for the notification template.';


--
-- Name: COLUMN templates.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.name IS 'Name of the template.';


--
-- Name: COLUMN templates.template_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.template_type IS 'Categorization of the template (email or sms).';


--
-- Name: COLUMN templates.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.created_at IS 'Date and time when the template was created.';


--
-- Name: COLUMN templates.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.updated_at IS 'Date and time when the template was last modified.';


--
-- Name: COLUMN templates.service_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.service_id IS 'Foreign key reference to the service that owns this template.';


--
-- Name: COLUMN templates.created_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.created_by_id IS 'Foreign key reference to the user who created the template.';


--
-- Name: COLUMN templates.version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.version IS 'Version number of the template.';


--
-- Name: COLUMN templates.archived; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.archived IS 'Indicates whether the template has been archived and is no longer available for new notifications.';


--
-- Name: COLUMN templates.process_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.process_type IS 'Defines the processing priority of notification sent using this template.';


--
-- Name: COLUMN templates.hidden; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.hidden IS 'Indicates whether the template should be hidden in the interface.';


--
-- Name: COLUMN templates.template_category_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.template_category_id IS 'Foreign key reference to the category this template belongs to.';


--
-- Name: COLUMN templates.text_direction_rtl; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates.text_direction_rtl IS 'Indicates whether the template content should be rendered right-to-left.';


--
-- Name: templates_history; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN templates_history.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.id IS 'Unique identifier for the notification template.';


--
-- Name: COLUMN templates_history.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.name IS 'Name of the template.';


--
-- Name: COLUMN templates_history.template_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.template_type IS 'Categorization of the template (email or sms).';


--
-- Name: COLUMN templates_history.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.created_at IS 'Date and time when the template was created.';


--
-- Name: COLUMN templates_history.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.updated_at IS 'Date and time when the template was last modified.';


--
-- Name: COLUMN templates_history.service_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.service_id IS 'Foreign key reference to the service that owns this template.';


--
-- Name: COLUMN templates_history.created_by_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.created_by_id IS 'Foreign key reference to the user who created the template.';


--
-- Name: COLUMN templates_history.version; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.version IS 'Version number of the template.';


--
-- Name: COLUMN templates_history.archived; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.archived IS 'Indicates whether the template has been archived and is no longer available for new notifications.';


--
-- Name: COLUMN templates_history.process_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.process_type IS 'Defines the processing priority of notification sent using this template.';


--
-- Name: COLUMN templates_history.hidden; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.hidden IS 'Indicates whether the template should be hidden in the interface.';


--
-- Name: COLUMN templates_history.template_category_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.template_category_id IS 'Foreign key reference to the category this template belongs to.';


--
-- Name: COLUMN templates_history.text_direction_rtl; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.templates_history.text_direction_rtl IS 'Indicates whether the template content should be rendered right-to-left.';


--
-- Name: user_folder_permissions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_folder_permissions (
    user_id uuid NOT NULL,
    template_folder_id uuid NOT NULL,
    service_id uuid NOT NULL
);


ALTER TABLE public.user_folder_permissions OWNER TO postgres;

--
-- Name: user_to_organisation; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_to_organisation (
    user_id uuid,
    organisation_id uuid
);


ALTER TABLE public.user_to_organisation OWNER TO postgres;

--
-- Name: COLUMN user_to_organisation.user_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.user_to_organisation.user_id IS 'Foreign key reference for the user linked to the organisation.';


--
-- Name: COLUMN user_to_organisation.organisation_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.user_to_organisation.organisation_id IS 'Foreign key reference for the organisation linked to the user.';


--
-- Name: user_to_service; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.user_to_service (
    user_id uuid,
    service_id uuid
);


ALTER TABLE public.user_to_service OWNER TO postgres;

--
-- Name: COLUMN user_to_service.user_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.user_to_service.user_id IS 'Foreign key reference for the user linked to the service.';


--
-- Name: COLUMN user_to_service.service_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.user_to_service.service_id IS 'Foreign key reference for the service linked to the user.';


--
-- Name: users; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: COLUMN users.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.id IS 'Unique identifier for the user account.';


--
-- Name: COLUMN users.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.created_at IS 'Date and time when the user account was created.';


--
-- Name: COLUMN users.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.updated_at IS 'Date and time when the user account was last modified.';


--
-- Name: COLUMN users.password_changed_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.password_changed_at IS 'Date and time when the user last changed their password.';


--
-- Name: COLUMN users.logged_in_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.logged_in_at IS 'Date and time of the user''s most recent successful login.';


--
-- Name: COLUMN users.failed_login_count; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.failed_login_count IS 'Number of consecutive failed login attempts for this user account.';


--
-- Name: COLUMN users.state; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.state IS 'Current status of the user account (active, inactive, pending).';


--
-- Name: COLUMN users.platform_admin; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.platform_admin IS 'Indicates whether the user has system-wide administrator privileges.';


--
-- Name: COLUMN users.auth_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.auth_type IS '2FA authentication method used by this user (email_auth, sms_auth).';


--
-- Name: COLUMN users.blocked; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.blocked IS 'Indicates whether the user is currently blocked from accessing the system.';


--
-- Name: COLUMN users.password_expired; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.users.password_expired IS 'Indicates whether the user''s password has expired and needs to be reset.';


--
-- Name: verify_codes; Type: TABLE; Schema: public; Owner: postgres
--

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

--
-- Name: alembic_version alembic_version_pkc; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.alembic_version
    ADD CONSTRAINT alembic_version_pkc PRIMARY KEY (version_num);


--
-- Name: annual_billing annual_billing_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.annual_billing
    ADD CONSTRAINT annual_billing_pkey PRIMARY KEY (id);


--
-- Name: api_keys_history api_keys_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.api_keys_history
    ADD CONSTRAINT api_keys_history_pkey PRIMARY KEY (id, version);


--
-- Name: api_keys api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);


--
-- Name: api_keys api_keys_secret_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_secret_key UNIQUE (secret);


--
-- Name: auth_type auth_type_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.auth_type
    ADD CONSTRAINT auth_type_pkey PRIMARY KEY (name);


--
-- Name: branding_type branding_type_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.branding_type
    ADD CONSTRAINT branding_type_pkey PRIMARY KEY (name);


--
-- Name: complaints complaints_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.complaints
    ADD CONSTRAINT complaints_pkey PRIMARY KEY (id);


--
-- Name: daily_sorted_letter daily_sorted_letter_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.daily_sorted_letter
    ADD CONSTRAINT daily_sorted_letter_pkey PRIMARY KEY (id);


--
-- Name: dm_datetime dm_datetime_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.dm_datetime
    ADD CONSTRAINT dm_datetime_pkey PRIMARY KEY (bst_date);


--
-- Name: domain domain_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.domain
    ADD CONSTRAINT domain_pkey PRIMARY KEY (domain);


--
-- Name: email_branding email_branding_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_pkey PRIMARY KEY (id);


--
-- Name: events events_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);


--
-- Name: fido2_keys fido2_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.fido2_keys
    ADD CONSTRAINT fido2_keys_pkey PRIMARY KEY (id);


--
-- Name: fido2_sessions fido2_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.fido2_sessions
    ADD CONSTRAINT fido2_sessions_pkey PRIMARY KEY (user_id);


--
-- Name: ft_billing ft_billing_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ft_billing
    ADD CONSTRAINT ft_billing_pkey PRIMARY KEY (bst_date, template_id, service_id, notification_type, provider, rate_multiplier, international, rate, postage, sms_sending_vehicle);


--
-- Name: ft_notification_status ft_notification_status_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ft_notification_status
    ADD CONSTRAINT ft_notification_status_pkey PRIMARY KEY (bst_date, template_id, service_id, job_id, notification_type, key_type, notification_status);


--
-- Name: inbound_numbers inbound_numbers_number_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.inbound_numbers
    ADD CONSTRAINT inbound_numbers_number_key UNIQUE (number);


--
-- Name: inbound_numbers inbound_numbers_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.inbound_numbers
    ADD CONSTRAINT inbound_numbers_pkey PRIMARY KEY (id);


--
-- Name: inbound_sms inbound_sms_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.inbound_sms
    ADD CONSTRAINT inbound_sms_pkey PRIMARY KEY (id);


--
-- Name: invite_status_type invite_status_type_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invite_status_type
    ADD CONSTRAINT invite_status_type_pkey PRIMARY KEY (name);


--
-- Name: invited_organisation_users invited_organisation_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_pkey PRIMARY KEY (id);


--
-- Name: invited_users invited_users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_pkey PRIMARY KEY (id);


--
-- Name: template_folder ix_id_service_id; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT ix_id_service_id UNIQUE (id, service_id);


--
-- Name: job_status job_status_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.job_status
    ADD CONSTRAINT job_status_pkey PRIMARY KEY (name);


--
-- Name: jobs jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_pkey PRIMARY KEY (id);


--
-- Name: key_types key_types_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.key_types
    ADD CONSTRAINT key_types_pkey PRIMARY KEY (name);


--
-- Name: letter_branding letter_branding_filename_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.letter_branding
    ADD CONSTRAINT letter_branding_filename_key UNIQUE (filename);


--
-- Name: letter_branding letter_branding_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.letter_branding
    ADD CONSTRAINT letter_branding_name_key UNIQUE (name);


--
-- Name: letter_branding letter_branding_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.letter_branding
    ADD CONSTRAINT letter_branding_pkey PRIMARY KEY (id);


--
-- Name: letter_rates letter_rates_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.letter_rates
    ADD CONSTRAINT letter_rates_pkey PRIMARY KEY (id);


--
-- Name: login_events login_events_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.login_events
    ADD CONSTRAINT login_events_pkey PRIMARY KEY (id);


--
-- Name: monthly_notification_stats_summary monthly_notification_stats_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.monthly_notification_stats_summary
    ADD CONSTRAINT monthly_notification_stats_pkey PRIMARY KEY (month, service_id, notification_type);


--
-- Name: notification_history notification_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_pkey PRIMARY KEY (id);


--
-- Name: notification_status_types notification_status_types_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_status_types
    ADD CONSTRAINT notification_status_types_pkey PRIMARY KEY (name);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: organisation organisation_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT organisation_pkey PRIMARY KEY (id);


--
-- Name: organisation_types organisation_types_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organisation_types
    ADD CONSTRAINT organisation_types_pkey PRIMARY KEY (name);


--
-- Name: permissions permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_pkey PRIMARY KEY (id);


--
-- Name: provider_details_history provider_details_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.provider_details_history
    ADD CONSTRAINT provider_details_history_pkey PRIMARY KEY (id, version);


--
-- Name: provider_details provider_details_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.provider_details
    ADD CONSTRAINT provider_details_pkey PRIMARY KEY (id);


--
-- Name: provider_rates provider_rates_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.provider_rates
    ADD CONSTRAINT provider_rates_pkey PRIMARY KEY (id);


--
-- Name: rates rates_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.rates
    ADD CONSTRAINT rates_pkey PRIMARY KEY (id);


--
-- Name: reports reports_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_pkey PRIMARY KEY (id);


--
-- Name: scheduled_notifications scheduled_notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.scheduled_notifications
    ADD CONSTRAINT scheduled_notifications_pkey PRIMARY KEY (id);


--
-- Name: service_callback_api_history service_callback_api_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_callback_api_history
    ADD CONSTRAINT service_callback_api_history_pkey PRIMARY KEY (id, version);


--
-- Name: service_callback_api service_callback_api_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_pkey PRIMARY KEY (id);


--
-- Name: service_callback_type service_callback_type_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_callback_type
    ADD CONSTRAINT service_callback_type_pkey PRIMARY KEY (name);


--
-- Name: service_data_retention service_data_retention_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_data_retention
    ADD CONSTRAINT service_data_retention_pkey PRIMARY KEY (id);


--
-- Name: service_email_reply_to service_email_reply_to_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_email_reply_to
    ADD CONSTRAINT service_email_reply_to_pkey PRIMARY KEY (id);


--
-- Name: service_inbound_api_history service_inbound_api_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_inbound_api_history
    ADD CONSTRAINT service_inbound_api_history_pkey PRIMARY KEY (id, version);


--
-- Name: service_inbound_api service_inbound_api_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_inbound_api
    ADD CONSTRAINT service_inbound_api_pkey PRIMARY KEY (id);


--
-- Name: service_letter_branding service_letter_branding_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_letter_branding
    ADD CONSTRAINT service_letter_branding_pkey PRIMARY KEY (service_id);


--
-- Name: service_letter_contacts service_letter_contacts_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_letter_contacts
    ADD CONSTRAINT service_letter_contacts_pkey PRIMARY KEY (id);


--
-- Name: service_permission_types service_permission_types_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_permission_types
    ADD CONSTRAINT service_permission_types_pkey PRIMARY KEY (name);


--
-- Name: service_permissions service_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_permissions
    ADD CONSTRAINT service_permissions_pkey PRIMARY KEY (service_id, permission);


--
-- Name: service_sms_senders service_sms_senders_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_sms_senders
    ADD CONSTRAINT service_sms_senders_pkey PRIMARY KEY (id);


--
-- Name: service_safelist service_whitelist_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_safelist
    ADD CONSTRAINT service_whitelist_pkey PRIMARY KEY (id);


--
-- Name: services services_email_from_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_email_from_key UNIQUE (email_from);


--
-- Name: services_history services_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services_history
    ADD CONSTRAINT services_history_pkey PRIMARY KEY (id, version);


--
-- Name: services services_name_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_name_key UNIQUE (name);


--
-- Name: services services_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_pkey PRIMARY KEY (id);


--
-- Name: template_categories template_categories_name_en_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_name_en_key UNIQUE (name_en);


--
-- Name: template_categories template_categories_name_fr_key; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_name_fr_key UNIQUE (name_fr);


--
-- Name: template_categories template_categories_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_pkey PRIMARY KEY (id);


--
-- Name: template_folder_map template_folder_map_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_folder_map
    ADD CONSTRAINT template_folder_map_pkey PRIMARY KEY (template_id);


--
-- Name: template_folder template_folder_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT template_folder_pkey PRIMARY KEY (id);


--
-- Name: template_process_type template_process_type_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_process_type
    ADD CONSTRAINT template_process_type_pkey PRIMARY KEY (name);


--
-- Name: template_redacted template_redacted_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_redacted
    ADD CONSTRAINT template_redacted_pkey PRIMARY KEY (template_id);


--
-- Name: templates_history templates_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_pkey PRIMARY KEY (id, version);


--
-- Name: templates templates_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_pkey PRIMARY KEY (id);


--
-- Name: daily_sorted_letter uix_file_name_billing_day; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.daily_sorted_letter
    ADD CONSTRAINT uix_file_name_billing_day UNIQUE (file_name, billing_day);


--
-- Name: service_callback_api uix_service_callback_type; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT uix_service_callback_type UNIQUE (service_id, callback_type);


--
-- Name: service_data_retention uix_service_data_retention; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_data_retention
    ADD CONSTRAINT uix_service_data_retention UNIQUE (service_id, notification_type);


--
-- Name: service_email_branding uix_service_email_branding_one_per_service; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_email_branding
    ADD CONSTRAINT uix_service_email_branding_one_per_service PRIMARY KEY (service_id);


--
-- Name: permissions uix_service_user_permission; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT uix_service_user_permission UNIQUE (service_id, user_id, permission);


--
-- Name: user_to_organisation uix_user_to_organisation; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_to_organisation
    ADD CONSTRAINT uix_user_to_organisation UNIQUE (user_id, organisation_id);


--
-- Name: user_to_service uix_user_to_service; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_to_service
    ADD CONSTRAINT uix_user_to_service UNIQUE (user_id, service_id);


--
-- Name: email_branding uq_email_branding_name; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT uq_email_branding_name UNIQUE (name);


--
-- Name: annual_limits_data uq_service_id_notification_type_time_period; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.annual_limits_data
    ADD CONSTRAINT uq_service_id_notification_type_time_period UNIQUE (service_id, notification_type, time_period);


--
-- Name: user_folder_permissions user_folder_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_pkey PRIMARY KEY (user_id, template_folder_id, service_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: verify_codes verify_codes_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.verify_codes
    ADD CONSTRAINT verify_codes_pkey PRIMARY KEY (id);


--
-- Name: ix_annual_billing_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_annual_billing_service_id ON public.annual_billing USING btree (service_id);


--
-- Name: ix_api_keys_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_api_keys_created_by_id ON public.api_keys USING btree (created_by_id);


--
-- Name: ix_api_keys_history_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_api_keys_history_created_by_id ON public.api_keys_history USING btree (created_by_id);


--
-- Name: ix_api_keys_history_key_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_api_keys_history_key_type ON public.api_keys_history USING btree (key_type);


--
-- Name: ix_api_keys_history_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_api_keys_history_service_id ON public.api_keys_history USING btree (service_id);


--
-- Name: ix_api_keys_key_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_api_keys_key_type ON public.api_keys USING btree (key_type);


--
-- Name: ix_api_keys_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_api_keys_service_id ON public.api_keys USING btree (service_id);


--
-- Name: ix_complaints_notification_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_complaints_notification_id ON public.complaints USING btree (notification_id);


--
-- Name: ix_complaints_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_complaints_service_id ON public.complaints USING btree (service_id);


--
-- Name: ix_daily_sorted_letter_billing_day; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_daily_sorted_letter_billing_day ON public.daily_sorted_letter USING btree (billing_day);


--
-- Name: ix_daily_sorted_letter_file_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_daily_sorted_letter_file_name ON public.daily_sorted_letter USING btree (file_name);


--
-- Name: ix_dm_datetime_bst_date; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_dm_datetime_bst_date ON public.dm_datetime USING btree (bst_date);


--
-- Name: ix_dm_datetime_yearmonth; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_dm_datetime_yearmonth ON public.dm_datetime USING btree (year, month);


--
-- Name: ix_email_branding_brand_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_email_branding_brand_type ON public.email_branding USING btree (brand_type);


--
-- Name: ix_email_branding_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_email_branding_created_by_id ON public.email_branding USING btree (created_by_id);


--
-- Name: ix_email_branding_organisation_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_email_branding_organisation_id ON public.email_branding USING btree (organisation_id);


--
-- Name: ix_email_branding_updated_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_email_branding_updated_by_id ON public.email_branding USING btree (updated_by_id);


--
-- Name: ix_fido2_keys_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_fido2_keys_user_id ON public.fido2_keys USING btree (user_id);


--
-- Name: ix_ft_billing_bst_date; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_billing_bst_date ON public.ft_billing USING btree (bst_date);


--
-- Name: ix_ft_billing_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_billing_service_id ON public.ft_billing USING btree (service_id);


--
-- Name: ix_ft_billing_template_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_billing_template_id ON public.ft_billing USING btree (template_id);


--
-- Name: ix_ft_notification_service_bst; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_notification_service_bst ON public.ft_notification_status USING btree (service_id, bst_date);


--
-- Name: ix_ft_notification_status_bst_date; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_notification_status_bst_date ON public.ft_notification_status USING btree (bst_date);


--
-- Name: ix_ft_notification_status_job_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_notification_status_job_id ON public.ft_notification_status USING btree (job_id);


--
-- Name: ix_ft_notification_status_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_notification_status_service_id ON public.ft_notification_status USING btree (service_id);


--
-- Name: ix_ft_notification_status_stats_lookup; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_notification_status_stats_lookup ON public.ft_notification_status USING btree (bst_date, notification_status, key_type) INCLUDE (notification_type, notification_count);


--
-- Name: ix_ft_notification_status_template_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_ft_notification_status_template_id ON public.ft_notification_status USING btree (template_id);


--
-- Name: ix_inbound_numbers_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ix_inbound_numbers_service_id ON public.inbound_numbers USING btree (service_id);


--
-- Name: ix_inbound_sms_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_inbound_sms_service_id ON public.inbound_sms USING btree (service_id);


--
-- Name: ix_inbound_sms_user_number; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_inbound_sms_user_number ON public.inbound_sms USING btree (user_number);


--
-- Name: ix_invited_users_auth_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_invited_users_auth_type ON public.invited_users USING btree (auth_type);


--
-- Name: ix_invited_users_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_invited_users_service_id ON public.invited_users USING btree (service_id);


--
-- Name: ix_invited_users_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_invited_users_user_id ON public.invited_users USING btree (user_id);


--
-- Name: ix_jobs_api_key_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_api_key_id ON public.jobs USING btree (api_key_id);


--
-- Name: ix_jobs_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_created_at ON public.jobs USING btree (created_at);


--
-- Name: ix_jobs_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_created_by_id ON public.jobs USING btree (created_by_id);


--
-- Name: ix_jobs_job_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_job_status ON public.jobs USING btree (job_status);


--
-- Name: ix_jobs_processing_started; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_processing_started ON public.jobs USING btree (processing_started);


--
-- Name: ix_jobs_scheduled_for; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_scheduled_for ON public.jobs USING btree (scheduled_for);


--
-- Name: ix_jobs_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_service_id ON public.jobs USING btree (service_id);


--
-- Name: ix_jobs_template_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_jobs_template_id ON public.jobs USING btree (template_id);


--
-- Name: ix_login_events_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_login_events_user_id ON public.login_events USING btree (user_id);


--
-- Name: ix_monthly_notification_stats_notification_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_monthly_notification_stats_notification_type ON public.monthly_notification_stats_summary USING btree (notification_type);


--
-- Name: ix_monthly_notification_stats_updated_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_monthly_notification_stats_updated_at ON public.monthly_notification_stats_summary USING btree (updated_at);


--
-- Name: ix_notification_history_api_key_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_api_key_id ON public.notification_history USING btree (api_key_id);


--
-- Name: ix_notification_history_api_key_id_created; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_api_key_id_created ON public.notification_history USING btree (api_key_id, created_at);


--
-- Name: ix_notification_history_created_api_key_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_created_api_key_id ON public.notification_history USING btree (created_at, api_key_id);


--
-- Name: ix_notification_history_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_created_at ON public.notification_history USING btree (created_at);


--
-- Name: ix_notification_history_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_created_by_id ON public.notification_history USING btree (created_by_id);


--
-- Name: ix_notification_history_feedback_reason; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_feedback_reason ON public.notification_history USING btree (feedback_reason);


--
-- Name: ix_notification_history_feedback_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_feedback_type ON public.notification_history USING btree (feedback_type);


--
-- Name: ix_notification_history_job_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_job_id ON public.notification_history USING btree (job_id);


--
-- Name: ix_notification_history_key_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_key_type ON public.notification_history USING btree (key_type);


--
-- Name: ix_notification_history_notification_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_notification_status ON public.notification_history USING btree (notification_status);


--
-- Name: ix_notification_history_notification_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_notification_type ON public.notification_history USING btree (notification_type);


--
-- Name: ix_notification_history_reference; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_reference ON public.notification_history USING btree (reference);


--
-- Name: ix_notification_history_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_service_id ON public.notification_history USING btree (service_id);


--
-- Name: ix_notification_history_service_id_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_service_id_created_at ON public.notification_history USING btree (service_id, date(created_at));


--
-- Name: ix_notification_history_template_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_template_id ON public.notification_history USING btree (template_id);


--
-- Name: ix_notification_history_week_created; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notification_history_week_created ON public.notification_history USING btree (date_trunc('week'::text, created_at));


--
-- Name: ix_notifications_api_key_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_api_key_id ON public.notifications USING btree (api_key_id);


--
-- Name: ix_notifications_client_reference; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_client_reference ON public.notifications USING btree (client_reference);


--
-- Name: ix_notifications_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_created_at ON public.notifications USING btree (created_at);


--
-- Name: ix_notifications_feedback_reason; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_feedback_reason ON public.notifications USING btree (feedback_reason);


--
-- Name: ix_notifications_feedback_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_feedback_type ON public.notifications USING btree (feedback_type);


--
-- Name: ix_notifications_job_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_job_id ON public.notifications USING btree (job_id);


--
-- Name: ix_notifications_key_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_key_type ON public.notifications USING btree (key_type);


--
-- Name: ix_notifications_notification_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_notification_status ON public.notifications USING btree (notification_status);


--
-- Name: ix_notifications_notification_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_notification_type ON public.notifications USING btree (notification_type);


--
-- Name: ix_notifications_reference; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_reference ON public.notifications USING btree (reference);


--
-- Name: ix_notifications_service_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_service_created_at ON public.notifications USING btree (service_id, created_at);


--
-- Name: ix_notifications_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_service_id ON public.notifications USING btree (service_id);


--
-- Name: ix_notifications_service_id_created_at; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_service_id_created_at ON public.notifications USING btree (service_id, date(created_at));


--
-- Name: ix_notifications_template_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_notifications_template_id ON public.notifications USING btree (template_id);


--
-- Name: ix_organisation_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ix_organisation_name ON public.organisation USING btree (name);


--
-- Name: ix_permissions_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_permissions_service_id ON public.permissions USING btree (service_id);


--
-- Name: ix_permissions_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_permissions_user_id ON public.permissions USING btree (user_id);


--
-- Name: ix_provider_details_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_provider_details_created_by_id ON public.provider_details USING btree (created_by_id);


--
-- Name: ix_provider_details_history_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_provider_details_history_created_by_id ON public.provider_details_history USING btree (created_by_id);


--
-- Name: ix_provider_rates_provider_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_provider_rates_provider_id ON public.provider_rates USING btree (provider_id);


--
-- Name: ix_rates_notification_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_rates_notification_type ON public.rates USING btree (notification_type);


--
-- Name: ix_reports_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_reports_service_id ON public.reports USING btree (service_id);


--
-- Name: ix_scheduled_notifications_notification_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_scheduled_notifications_notification_id ON public.scheduled_notifications USING btree (notification_id);


--
-- Name: ix_service_callback_api_history_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_callback_api_history_service_id ON public.service_callback_api_history USING btree (service_id);


--
-- Name: ix_service_callback_api_history_updated_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_callback_api_history_updated_by_id ON public.service_callback_api_history USING btree (updated_by_id);


--
-- Name: ix_service_callback_api_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_callback_api_service_id ON public.service_callback_api USING btree (service_id);


--
-- Name: ix_service_callback_api_updated_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_callback_api_updated_by_id ON public.service_callback_api USING btree (updated_by_id);


--
-- Name: ix_service_data_retention_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_data_retention_service_id ON public.service_data_retention USING btree (service_id);


--
-- Name: ix_service_email_reply_to_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_email_reply_to_service_id ON public.service_email_reply_to USING btree (service_id);


--
-- Name: ix_service_history_sensitive_service; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_history_sensitive_service ON public.services_history USING btree (sensitive_service);


--
-- Name: ix_service_id_notification_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_id_notification_type ON public.annual_limits_data USING btree (service_id, notification_type);


--
-- Name: ix_service_id_notification_type_time; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_id_notification_type_time ON public.annual_limits_data USING btree (time_period, service_id, notification_type);


--
-- Name: ix_service_inbound_api_history_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_inbound_api_history_service_id ON public.service_inbound_api_history USING btree (service_id);


--
-- Name: ix_service_inbound_api_history_updated_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_inbound_api_history_updated_by_id ON public.service_inbound_api_history USING btree (updated_by_id);


--
-- Name: ix_service_inbound_api_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ix_service_inbound_api_service_id ON public.service_inbound_api USING btree (service_id);


--
-- Name: ix_service_inbound_api_updated_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_inbound_api_updated_by_id ON public.service_inbound_api USING btree (updated_by_id);


--
-- Name: ix_service_letter_contacts_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_letter_contacts_service_id ON public.service_letter_contacts USING btree (service_id);


--
-- Name: ix_service_permissions_permission; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_permissions_permission ON public.service_permissions USING btree (permission);


--
-- Name: ix_service_permissions_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_permissions_service_id ON public.service_permissions USING btree (service_id);


--
-- Name: ix_service_sensitive_service; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_sensitive_service ON public.services USING btree (sensitive_service);


--
-- Name: ix_service_sms_senders_inbound_number_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ix_service_sms_senders_inbound_number_id ON public.service_sms_senders USING btree (inbound_number_id);


--
-- Name: ix_service_sms_senders_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_sms_senders_service_id ON public.service_sms_senders USING btree (service_id);


--
-- Name: ix_service_whitelist_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_service_whitelist_service_id ON public.service_safelist USING btree (service_id);


--
-- Name: ix_services_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_services_created_by_id ON public.services USING btree (created_by_id);


--
-- Name: ix_services_history_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_services_history_created_by_id ON public.services_history USING btree (created_by_id);


--
-- Name: ix_services_history_organisation_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_services_history_organisation_id ON public.services_history USING btree (organisation_id);


--
-- Name: ix_services_organisation_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_services_organisation_id ON public.services USING btree (organisation_id);


--
-- Name: ix_template_categories_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_template_categories_created_by_id ON public.template_categories USING btree (created_by_id);


--
-- Name: ix_template_categories_name_en; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_template_categories_name_en ON public.template_categories USING btree (name_en);


--
-- Name: ix_template_categories_name_fr; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_template_categories_name_fr ON public.template_categories USING btree (name_fr);


--
-- Name: ix_template_categories_updated_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_template_categories_updated_by_id ON public.template_categories USING btree (updated_by_id);


--
-- Name: ix_template_category_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_template_category_id ON public.templates USING btree (template_category_id);


--
-- Name: ix_template_redacted_updated_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_template_redacted_updated_by_id ON public.template_redacted USING btree (updated_by_id);


--
-- Name: ix_templates_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_templates_created_by_id ON public.templates USING btree (created_by_id);


--
-- Name: ix_templates_history_created_by_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_templates_history_created_by_id ON public.templates_history USING btree (created_by_id);


--
-- Name: ix_templates_history_process_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_templates_history_process_type ON public.templates_history USING btree (process_type);


--
-- Name: ix_templates_history_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_templates_history_service_id ON public.templates_history USING btree (service_id);


--
-- Name: ix_templates_process_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_templates_process_type ON public.templates USING btree (process_type);


--
-- Name: ix_templates_service_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_templates_service_id ON public.templates USING btree (service_id);


--
-- Name: ix_users_auth_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_users_auth_type ON public.users USING btree (auth_type);


--
-- Name: ix_users_email_address; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX ix_users_email_address ON public.users USING btree (email_address);


--
-- Name: ix_users_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_users_name ON public.users USING btree (name);


--
-- Name: ix_verify_codes_user_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX ix_verify_codes_user_id ON public.verify_codes USING btree (user_id);


--
-- Name: uix_service_to_key_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX uix_service_to_key_name ON public.api_keys USING btree (service_id, name) WHERE (expiry_date IS NULL);


--
-- Name: annual_billing annual_billing_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.annual_billing
    ADD CONSTRAINT annual_billing_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: api_keys api_keys_key_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_key_type_fkey FOREIGN KEY (key_type) REFERENCES public.key_types(name);


--
-- Name: api_keys api_keys_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: complaints complaints_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.complaints
    ADD CONSTRAINT complaints_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: domain domain_organisation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.domain
    ADD CONSTRAINT domain_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);


--
-- Name: email_branding email_branding_brand_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_brand_type_fkey FOREIGN KEY (brand_type) REFERENCES public.branding_type(name);


--
-- Name: email_branding email_branding_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: email_branding email_branding_updated_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT email_branding_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);


--
-- Name: fido2_keys fido2_keys_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.fido2_keys
    ADD CONSTRAINT fido2_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: fido2_sessions fido2_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.fido2_sessions
    ADD CONSTRAINT fido2_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: api_keys fk_api_keys_created_by_id; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT fk_api_keys_created_by_id FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: email_branding fk_email_branding_organisation; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.email_branding
    ADD CONSTRAINT fk_email_branding_organisation FOREIGN KEY (organisation_id) REFERENCES public.organisation(id) ON DELETE SET NULL;


--
-- Name: notification_history fk_notification_history_notification_status; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT fk_notification_history_notification_status FOREIGN KEY (notification_status) REFERENCES public.notification_status_types(name);


--
-- Name: notifications fk_notifications_notification_status; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT fk_notifications_notification_status FOREIGN KEY (notification_status) REFERENCES public.notification_status_types(name);


--
-- Name: organisation fk_organisation_agreement_user_id; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT fk_organisation_agreement_user_id FOREIGN KEY (agreement_signed_by_id) REFERENCES public.users(id);


--
-- Name: organisation fk_organisation_letter_branding_id; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT fk_organisation_letter_branding_id FOREIGN KEY (letter_branding_id) REFERENCES public.letter_branding(id);


--
-- Name: annual_limits_data fk_service_id; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.annual_limits_data
    ADD CONSTRAINT fk_service_id FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: services fk_service_organisation; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT fk_service_organisation FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);


--
-- Name: services fk_services_created_by_id; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT fk_services_created_by_id FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: services fk_services_go_live_user; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT fk_services_go_live_user FOREIGN KEY (go_live_user_id) REFERENCES public.users(id);


--
-- Name: templates fk_template_template_categories; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT fk_template_template_categories FOREIGN KEY (template_category_id) REFERENCES public.template_categories(id);


--
-- Name: templates fk_templates_created_by_id; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT fk_templates_created_by_id FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: inbound_numbers inbound_numbers_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.inbound_numbers
    ADD CONSTRAINT inbound_numbers_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: inbound_sms inbound_sms_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.inbound_sms
    ADD CONSTRAINT inbound_sms_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: invited_organisation_users invited_organisation_users_invited_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_invited_by_id_fkey FOREIGN KEY (invited_by_id) REFERENCES public.users(id);


--
-- Name: invited_organisation_users invited_organisation_users_organisation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);


--
-- Name: invited_organisation_users invited_organisation_users_status_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_organisation_users
    ADD CONSTRAINT invited_organisation_users_status_fkey FOREIGN KEY (status) REFERENCES public.invite_status_type(name);


--
-- Name: invited_users invited_users_auth_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_auth_type_fkey FOREIGN KEY (auth_type) REFERENCES public.auth_type(name);


--
-- Name: invited_users invited_users_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: invited_users invited_users_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.invited_users
    ADD CONSTRAINT invited_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: jobs jobs_api_keys_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_api_keys_id_fkey FOREIGN KEY (api_key_id) REFERENCES public.api_keys(id);


--
-- Name: jobs jobs_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: jobs jobs_job_status_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_job_status_fkey FOREIGN KEY (job_status) REFERENCES public.job_status(name);


--
-- Name: jobs jobs_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: jobs jobs_template_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.jobs
    ADD CONSTRAINT jobs_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.templates(id);


--
-- Name: login_events login_events_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.login_events
    ADD CONSTRAINT login_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: notification_history notification_history_api_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_api_key_id_fkey FOREIGN KEY (api_key_id) REFERENCES public.api_keys(id);


--
-- Name: notification_history notification_history_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id);


--
-- Name: notification_history notification_history_key_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_key_type_fkey FOREIGN KEY (key_type) REFERENCES public.key_types(name);


--
-- Name: notification_history notification_history_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: notification_history notification_history_templates_history_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notification_history
    ADD CONSTRAINT notification_history_templates_history_fkey FOREIGN KEY (template_id, template_version) REFERENCES public.templates_history(id, version);


--
-- Name: notifications notifications_api_key_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_api_key_id_fkey FOREIGN KEY (api_key_id) REFERENCES public.api_keys(id);


--
-- Name: notifications notifications_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: notifications notifications_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id);


--
-- Name: notifications notifications_key_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_key_type_fkey FOREIGN KEY (key_type) REFERENCES public.key_types(name);


--
-- Name: notifications notifications_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: notifications notifications_templates_history_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_templates_history_fkey FOREIGN KEY (template_id, template_version) REFERENCES public.templates_history(id, version);


--
-- Name: organisation organisation_organisation_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.organisation
    ADD CONSTRAINT organisation_organisation_type_fkey FOREIGN KEY (organisation_type) REFERENCES public.organisation_types(name);


--
-- Name: permissions permissions_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: permissions permissions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.permissions
    ADD CONSTRAINT permissions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: provider_details provider_details_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.provider_details
    ADD CONSTRAINT provider_details_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: provider_details_history provider_details_history_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.provider_details_history
    ADD CONSTRAINT provider_details_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: provider_rates provider_rate_to_provider_fk; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.provider_rates
    ADD CONSTRAINT provider_rate_to_provider_fk FOREIGN KEY (provider_id) REFERENCES public.provider_details(id);


--
-- Name: reports reports_job_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_job_id_fkey FOREIGN KEY (job_id) REFERENCES public.jobs(id);


--
-- Name: reports reports_requesting_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_requesting_user_id_fkey FOREIGN KEY (requesting_user_id) REFERENCES public.users(id);


--
-- Name: reports reports_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.reports
    ADD CONSTRAINT reports_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: scheduled_notifications scheduled_notifications_notification_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.scheduled_notifications
    ADD CONSTRAINT scheduled_notifications_notification_id_fkey FOREIGN KEY (notification_id) REFERENCES public.notifications(id);


--
-- Name: service_callback_api service_callback_api_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_callback_api service_callback_api_type_fk; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_type_fk FOREIGN KEY (callback_type) REFERENCES public.service_callback_type(name);


--
-- Name: service_callback_api service_callback_api_updated_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_callback_api
    ADD CONSTRAINT service_callback_api_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);


--
-- Name: service_data_retention service_data_retention_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_data_retention
    ADD CONSTRAINT service_data_retention_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_email_branding service_email_branding_email_branding_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_email_branding
    ADD CONSTRAINT service_email_branding_email_branding_id_fkey FOREIGN KEY (email_branding_id) REFERENCES public.email_branding(id);


--
-- Name: service_email_branding service_email_branding_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_email_branding
    ADD CONSTRAINT service_email_branding_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_email_reply_to service_email_reply_to_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_email_reply_to
    ADD CONSTRAINT service_email_reply_to_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_inbound_api service_inbound_api_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_inbound_api
    ADD CONSTRAINT service_inbound_api_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_inbound_api service_inbound_api_updated_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_inbound_api
    ADD CONSTRAINT service_inbound_api_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);


--
-- Name: service_letter_branding service_letter_branding_letter_branding_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_letter_branding
    ADD CONSTRAINT service_letter_branding_letter_branding_id_fkey FOREIGN KEY (letter_branding_id) REFERENCES public.letter_branding(id);


--
-- Name: service_letter_branding service_letter_branding_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_letter_branding
    ADD CONSTRAINT service_letter_branding_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_letter_contacts service_letter_contacts_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_letter_contacts
    ADD CONSTRAINT service_letter_contacts_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_permissions service_permissions_permission_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_permissions
    ADD CONSTRAINT service_permissions_permission_fkey FOREIGN KEY (permission) REFERENCES public.service_permission_types(name);


--
-- Name: service_permissions service_permissions_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_permissions
    ADD CONSTRAINT service_permissions_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_sms_senders service_sms_senders_inbound_number_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_sms_senders
    ADD CONSTRAINT service_sms_senders_inbound_number_id_fkey FOREIGN KEY (inbound_number_id) REFERENCES public.inbound_numbers(id);


--
-- Name: service_sms_senders service_sms_senders_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_sms_senders
    ADD CONSTRAINT service_sms_senders_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: service_safelist service_whitelist_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.service_safelist
    ADD CONSTRAINT service_whitelist_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: services_history services_history_suspended_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services_history
    ADD CONSTRAINT services_history_suspended_by_id_fkey FOREIGN KEY (suspended_by_id) REFERENCES public.users(id);


--
-- Name: services services_organisation_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_organisation_type_fkey FOREIGN KEY (organisation_type) REFERENCES public.organisation_types(name);


--
-- Name: services services_suspended_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.services
    ADD CONSTRAINT services_suspended_by_id_fkey FOREIGN KEY (suspended_by_id) REFERENCES public.users(id);


--
-- Name: template_categories template_categories_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_categories
    ADD CONSTRAINT template_categories_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: template_folder_map template_folder_map_template_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_folder_map
    ADD CONSTRAINT template_folder_map_template_folder_id_fkey FOREIGN KEY (template_folder_id) REFERENCES public.template_folder(id);


--
-- Name: template_folder_map template_folder_map_template_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_folder_map
    ADD CONSTRAINT template_folder_map_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.templates(id);


--
-- Name: template_folder template_folder_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT template_folder_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.template_folder(id);


--
-- Name: template_folder template_folder_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_folder
    ADD CONSTRAINT template_folder_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: template_redacted template_redacted_template_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_redacted
    ADD CONSTRAINT template_redacted_template_id_fkey FOREIGN KEY (template_id) REFERENCES public.templates(id);


--
-- Name: template_redacted template_redacted_updated_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.template_redacted
    ADD CONSTRAINT template_redacted_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id);


--
-- Name: templates_history templates_history_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id);


--
-- Name: templates_history templates_history_process_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_process_type_fkey FOREIGN KEY (process_type) REFERENCES public.template_process_type(name);


--
-- Name: templates_history templates_history_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: templates_history templates_history_service_letter_contact_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates_history
    ADD CONSTRAINT templates_history_service_letter_contact_id_fkey FOREIGN KEY (service_letter_contact_id) REFERENCES public.service_letter_contacts(id);


--
-- Name: templates templates_process_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_process_type_fkey FOREIGN KEY (process_type) REFERENCES public.template_process_type(name);


--
-- Name: templates templates_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: templates templates_service_letter_contact_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.templates
    ADD CONSTRAINT templates_service_letter_contact_id_fkey FOREIGN KEY (service_letter_contact_id) REFERENCES public.service_letter_contacts(id);


--
-- Name: user_folder_permissions user_folder_permissions_template_folder_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_template_folder_id_fkey FOREIGN KEY (template_folder_id) REFERENCES public.template_folder(id);


--
-- Name: user_folder_permissions user_folder_permissions_template_folder_id_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_template_folder_id_service_id_fkey FOREIGN KEY (template_folder_id, service_id) REFERENCES public.template_folder(id, service_id);


--
-- Name: user_folder_permissions user_folder_permissions_user_id_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_folder_permissions
    ADD CONSTRAINT user_folder_permissions_user_id_service_id_fkey FOREIGN KEY (user_id, service_id) REFERENCES public.user_to_service(user_id, service_id);


--
-- Name: user_to_organisation user_to_organisation_organisation_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_to_organisation
    ADD CONSTRAINT user_to_organisation_organisation_id_fkey FOREIGN KEY (organisation_id) REFERENCES public.organisation(id);


--
-- Name: user_to_organisation user_to_organisation_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_to_organisation
    ADD CONSTRAINT user_to_organisation_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: user_to_service user_to_service_service_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_to_service
    ADD CONSTRAINT user_to_service_service_id_fkey FOREIGN KEY (service_id) REFERENCES public.services(id);


--
-- Name: user_to_service user_to_service_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.user_to_service
    ADD CONSTRAINT user_to_service_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: users users_auth_type_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_auth_type_fkey FOREIGN KEY (auth_type) REFERENCES public.auth_type(name);


--
-- Name: verify_codes verify_codes_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.verify_codes
    ADD CONSTRAINT verify_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- PostgreSQL database dump complete
--

\unrestrict eR9wjtmhT4bdsi7OfU5qb8707ivNySeaj09oFXGEA2LdhuPPDYWY9DPhQOljn6z

