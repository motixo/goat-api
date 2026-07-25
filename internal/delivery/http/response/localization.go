package response

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"

	"golang.org/x/text/language"
)

type TranslationKey string

type TranslationParams map[string]any

var errInvalidLocalizationAssets = errors.New("invalid HTTP problem localization assets")

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

var translationParameterDefinitions = map[TranslationKey][]string{
	DetailInvalidUserRole:   {"Role"},
	DetailRateLimitExceeded: {"RetryAfter"},
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
	problemLocalizer, problemLocalizerErr = loadLocalizer(englishCatalogJSON, persianCatalogJSON)
)

func resolveLocale(acceptLanguage string) locale {
	_, index := language.MatchStrings(problemLocaleMatcher, acceptLanguage)
	if index == 1 {
		return localePersian
	}
	return localeEnglish
}

// ValidateRuntimeAssets verifies the embedded HTTP problem catalogs without
// exposing catalog messages in returned errors. Bootstrap calls this before
// constructing any HTTP runtime resources.
func ValidateRuntimeAssets() error {
	if problemLocalizerErr != nil {
		return fmt.Errorf("validate embedded HTTP problem translations: %w", problemLocalizerErr)
	}
	return nil
}

func loadLocalizer(englishData, persianData []byte) (localizer, error) {
	if err := validateTranslationContract(); err != nil {
		return localizer{}, err
	}

	englishCatalog, err := loadCatalog(localeEnglish, englishData)
	if err != nil {
		return localizer{}, err
	}
	persianCatalog, err := loadCatalog(localePersian, persianData)
	if err != nil {
		return localizer{}, err
	}

	for _, key := range translationKeys {
		englishTemplate, exists := englishCatalog[key]
		if !exists {
			return localizer{}, localizationAssetError(
				"English catalog is missing required key %q",
				key,
			)
		}
		if err := validateTemplateParameters(
			localeEnglish,
			key,
			englishTemplate,
			translationParameterDefinitions[key],
		); err != nil {
			return localizer{}, err
		}
	}

	for key, persianTemplate := range persianCatalog {
		englishTemplate, exists := englishCatalog[key]
		if !exists {
			return localizer{}, localizationAssetError(
				"Persian catalog contains a key with no English definition",
			)
		}
		englishParameters := templateParameters(englishTemplate)
		if err := validateTemplateParameters(
			localePersian,
			key,
			persianTemplate,
			englishParameters,
		); err != nil {
			return localizer{}, err
		}
	}

	return localizer{catalogs: map[locale]translationCatalog{
		localeEnglish: englishCatalog,
		localePersian: persianCatalog,
	}}, nil
}

func validateTranslationContract() error {
	seenKeys := make(map[TranslationKey]struct{}, len(translationKeys))
	for _, key := range translationKeys {
		if strings.TrimSpace(string(key)) == "" {
			return localizationAssetError("required translation key is empty")
		}
		if _, duplicate := seenKeys[key]; duplicate {
			return localizationAssetError("required translation key is registered more than once")
		}
		seenKeys[key] = struct{}{}
	}

	for key, parameters := range translationParameterDefinitions {
		if _, required := seenKeys[key]; !required {
			return localizationAssetError("parameter definition references an unregistered translation key")
		}
		seenParameters := make(map[string]struct{}, len(parameters))
		for _, parameter := range parameters {
			if strings.TrimSpace(parameter) == "" {
				return localizationAssetError("translation parameter name is empty")
			}
			if _, duplicate := seenParameters[parameter]; duplicate {
				return localizationAssetError("translation parameter is defined more than once")
			}
			seenParameters[parameter] = struct{}{}
		}
	}
	return nil
}

func loadCatalog(catalogLocale locale, data []byte) (translationCatalog, error) {
	messages, err := decodeCatalogMessages(catalogLocale, data)
	if err != nil {
		return nil, err
	}
	catalog := make(translationCatalog, len(messages))
	for key, message := range messages {
		parsed, err := template.New(string(key)).Option("missingkey=error").Parse(message)
		if err != nil {
			return nil, localizationAssetError(
				"%s catalog contains invalid template syntax for key %q",
				catalogLocale,
				key,
			)
		}
		catalog[key] = parsed
	}
	return catalog, nil
}

func decodeCatalogMessages(catalogLocale locale, data []byte) (map[TranslationKey]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, malformedCatalogError(catalogLocale)
	}
	opening, ok := token.(json.Delim)
	if !ok || opening != '{' {
		return nil, malformedCatalogError(catalogLocale)
	}

	messages := make(map[TranslationKey]string)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, malformedCatalogError(catalogLocale)
		}
		rawKey, ok := token.(string)
		if !ok {
			return nil, malformedCatalogError(catalogLocale)
		}
		if strings.TrimSpace(rawKey) == "" {
			return nil, localizationAssetError("%s catalog contains an empty translation key", catalogLocale)
		}

		key := TranslationKey(rawKey)
		if _, duplicate := messages[key]; duplicate {
			return nil, localizationAssetError("%s catalog contains a duplicate translation key", catalogLocale)
		}

		var message string
		if err := decoder.Decode(&message); err != nil {
			return nil, malformedCatalogError(catalogLocale)
		}
		if strings.TrimSpace(message) == "" {
			return nil, localizationAssetError("%s catalog contains an empty translation", catalogLocale)
		}
		messages[key] = message
	}

	token, err = decoder.Token()
	if err != nil {
		return nil, malformedCatalogError(catalogLocale)
	}
	closing, ok := token.(json.Delim)
	if !ok || closing != '}' {
		return nil, malformedCatalogError(catalogLocale)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, malformedCatalogError(catalogLocale)
	}
	return messages, nil
}

func validateTemplateParameters(
	catalogLocale locale,
	key TranslationKey,
	message *template.Template,
	expected []string,
) error {
	actual := templateParameters(message)
	if equalStringSets(actual, expected) {
		return nil
	}
	return localizationAssetError(
		"%s catalog has incompatible translation parameters for key %q",
		catalogLocale,
		key,
	)
}

func templateParameters(message *template.Template) []string {
	parameters := make(map[string]struct{})
	for _, associated := range message.Templates() {
		if associated.Tree != nil {
			collectTemplateParameters(associated.Root, parameters)
		}
	}

	result := make([]string, 0, len(parameters))
	for parameter := range parameters {
		result = append(result, parameter)
	}
	sort.Strings(result)
	return result
}

func collectTemplateParameters(node parse.Node, parameters map[string]struct{}) {
	if node == nil {
		return
	}

	switch current := node.(type) {
	case *parse.ListNode:
		for _, child := range current.Nodes {
			collectTemplateParameters(child, parameters)
		}
	case *parse.ActionNode:
		collectTemplateParameters(current.Pipe, parameters)
	case *parse.IfNode:
		collectBranchParameters(current.BranchNode, parameters)
	case *parse.RangeNode:
		collectBranchParameters(current.BranchNode, parameters)
	case *parse.WithNode:
		collectBranchParameters(current.BranchNode, parameters)
	case *parse.TemplateNode:
		collectTemplateParameters(current.Pipe, parameters)
	case *parse.PipeNode:
		for _, command := range current.Cmds {
			collectTemplateParameters(command, parameters)
		}
	case *parse.CommandNode:
		for _, argument := range current.Args {
			collectTemplateParameters(argument, parameters)
		}
	case *parse.FieldNode:
		if len(current.Ident) > 0 {
			parameters[strings.Join(current.Ident, ".")] = struct{}{}
		}
	case *parse.ChainNode:
		collectTemplateParameters(current.Node, parameters)
		if len(current.Field) > 0 {
			parameters[strings.Join(current.Field, ".")] = struct{}{}
		}
	}
}

func collectBranchParameters(branch parse.BranchNode, parameters map[string]struct{}) {
	collectTemplateParameters(branch.Pipe, parameters)
	collectTemplateParameters(branch.List, parameters)
	collectTemplateParameters(branch.ElseList, parameters)
}

func equalStringSets(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, value := range expected {
		expectedSet[value] = struct{}{}
	}
	for _, value := range actual {
		if _, exists := expectedSet[value]; !exists {
			return false
		}
	}
	return true
}

func malformedCatalogError(catalogLocale locale) error {
	return localizationAssetError("%s catalog contains malformed JSON", catalogLocale)
}

func localizationAssetError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errInvalidLocalizationAssets, fmt.Sprintf(format, args...))
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
