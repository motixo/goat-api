package response

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"

	"golang.org/x/text/language"
)

type TranslationKey string

type TranslationParams map[string]any

type locale string

const (
	localeEnglish locale = "en"
	localePersian locale = "fa"
)

const (
	titleBadRequest          TranslationKey = "title.bad_request"
	titleUnauthorized        TranslationKey = "title.unauthorized"
	titleForbidden           TranslationKey = "title.forbidden"
	titleNotFound            TranslationKey = "title.not_found"
	titleConflict            TranslationKey = "title.conflict"
	titleTooManyRequests     TranslationKey = "title.too_many_requests"
	titleInternalServerError TranslationKey = "title.internal_server_error"

	detailCurrentSessionNotFound         TranslationKey = "detail.current_session_not_found"
	detailPasswordTooShort               TranslationKey = "detail.password_too_short"
	detailPasswordTooLong                TranslationKey = "detail.password_too_long"
	detailPasswordPolicyViolation        TranslationKey = "detail.password_policy_violation"
	detailCurrentPasswordIncorrect       TranslationKey = "detail.current_password_incorrect"
	detailProcessingError                TranslationKey = "detail.processing_error"
	detailTokenExpired                   TranslationKey = "detail.token_expired"
	detailTokenInvalid                   TranslationKey = "detail.token_invalid"
	detailInvalidCredentials             TranslationKey = "detail.invalid_credentials"
	detailAccountSuspendedContactSupport TranslationKey = "detail.account_suspended_contact_support"
	detailResourceNotFound               TranslationKey = "detail.resource_not_found"
	detailEmailAlreadyExists             TranslationKey = "detail.email_already_exists"
	detailPasswordSameAsCurrent          TranslationKey = "detail.password_same_as_current"
	detailConflict                       TranslationKey = "detail.conflict"
	detailUnexpected                     TranslationKey = "detail.unexpected"
	DetailInvalidRequestPayload          TranslationKey = "detail.invalid_request_payload"
	DetailInvalidPaginationParams        TranslationKey = "detail.invalid_pagination_params"
	DetailAuthenticationContextMissing   TranslationKey = "detail.authentication_context_missing"
	DetailInvalidUserRole                TranslationKey = "detail.invalid_user_role"
	DetailMissingAuthorizationHeader     TranslationKey = "detail.missing_or_invalid_authorization_header"
	DetailAccessTokenRequired            TranslationKey = "detail.access_token_required"
	DetailTokenRevoked                   TranslationKey = "detail.token_revoked"
	DetailAccountNotActivated            TranslationKey = "detail.account_not_activated"
	DetailAccountSuspended               TranslationKey = "detail.account_suspended"
	DetailContactSupport                 TranslationKey = "detail.contact_support"
	DetailAuthenticationRequired         TranslationKey = "detail.authentication_required"
	DetailInvalidUserContext             TranslationKey = "detail.invalid_user_context"
	DetailInsufficientPermissions        TranslationKey = "detail.insufficient_permissions"
	DetailRateLimitExceeded              TranslationKey = "detail.rate_limit_exceeded"
)

var translationKeys = []TranslationKey{
	titleBadRequest,
	titleUnauthorized,
	titleForbidden,
	titleNotFound,
	titleConflict,
	titleTooManyRequests,
	titleInternalServerError,
	detailCurrentSessionNotFound,
	detailPasswordTooShort,
	detailPasswordTooLong,
	detailPasswordPolicyViolation,
	detailCurrentPasswordIncorrect,
	detailProcessingError,
	detailTokenExpired,
	detailTokenInvalid,
	detailInvalidCredentials,
	detailAccountSuspendedContactSupport,
	detailResourceNotFound,
	detailEmailAlreadyExists,
	detailPasswordSameAsCurrent,
	detailConflict,
	detailUnexpected,
	DetailInvalidRequestPayload,
	DetailInvalidPaginationParams,
	DetailAuthenticationContextMissing,
	DetailInvalidUserRole,
	DetailMissingAuthorizationHeader,
	DetailAccessTokenRequired,
	DetailTokenRevoked,
	DetailAccountNotActivated,
	DetailAccountSuspended,
	DetailContactSupport,
	DetailAuthenticationRequired,
	DetailInvalidUserContext,
	DetailInsufficientPermissions,
	DetailRateLimitExceeded,
}

//go:embed translations/en.json
var englishCatalogJSON []byte

//go:embed translations/fa.json
var persianCatalogJSON []byte

type translationCatalog map[TranslationKey]*template.Template

type localizer struct {
	catalogs map[locale]translationCatalog
}

var (
	problemLocaleMatcher = language.NewMatcher([]language.Tag{
		language.English,
		language.Persian,
	})
	problemLocalizer = mustLoadLocalizer()
)

func resolveLocale(acceptLanguage string) locale {
	_, index := language.MatchStrings(problemLocaleMatcher, acceptLanguage)
	if index == 1 {
		return localePersian
	}
	return localeEnglish
}

func mustLoadLocalizer() localizer {
	catalogs := map[locale]translationCatalog{
		localeEnglish: mustLoadCatalog(localeEnglish, englishCatalogJSON),
		localePersian: mustLoadCatalog(localePersian, persianCatalogJSON),
	}
	for _, key := range translationKeys {
		if _, exists := catalogs[localeEnglish][key]; !exists {
			panic(fmt.Sprintf("English problem translation %q is missing", key))
		}
	}
	return localizer{catalogs: catalogs}
}

func mustLoadCatalog(catalogLocale locale, data []byte) translationCatalog {
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		panic(fmt.Sprintf("decode %s problem translations: %v", catalogLocale, err))
	}

	catalog := make(translationCatalog, len(messages))
	for rawKey, message := range messages {
		key := TranslationKey(rawKey)
		parsed, err := template.New(rawKey).Option("missingkey=error").Parse(message)
		if err != nil {
			panic(fmt.Sprintf("parse %s problem translation %q: %v", catalogLocale, key, err))
		}
		catalog[key] = parsed
	}
	return catalog
}

func (l localizer) translate(catalogLocale locale, key TranslationKey, params TranslationParams) string {
	translated, ok := l.execute(catalogLocale, key, params)
	if ok {
		return translated
	}
	if catalogLocale != localeEnglish {
		if translated, ok = l.execute(localeEnglish, key, params); ok {
			return translated
		}
	}
	translated, _ = l.execute(localeEnglish, detailUnexpected, nil)
	return translated
}

func (l localizer) execute(catalogLocale locale, key TranslationKey, params TranslationParams) (string, bool) {
	catalog, exists := l.catalogs[catalogLocale]
	if !exists {
		return "", false
	}
	message, exists := catalog[key]
	if !exists {
		return "", false
	}

	var rendered bytes.Buffer
	if err := message.Execute(&rendered, params); err != nil {
		return "", false
	}
	return rendered.String(), true
}
