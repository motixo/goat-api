package response

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

func TestEmbeddedProblemLocalizationAssetsAreValid(t *testing.T) {
	t.Parallel()

	if err := ValidateRuntimeAssets(); err != nil {
		t.Fatalf("ValidateRuntimeAssets() error = %v", err)
	}
}

func TestProblemLocalizationAssetValidationRejectsMalformedCatalogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		english []byte
		persian []byte
	}{
		{
			name:    "English",
			english: []byte(`{"title.bad_request":`),
			persian: persianCatalogJSON,
		},
		{
			name:    "Persian",
			english: englishCatalogJSON,
			persian: []byte(`{"title.bad_request":`),
		},
		{
			name:    "English template",
			english: catalogWithMessage(t, englishCatalogJSON, titleBadRequest, "{{"),
			persian: persianCatalogJSON,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := loadLocalizer(test.english, test.persian); !errors.Is(err, errInvalidLocalizationAssets) {
				t.Fatalf("loadLocalizer() error = %v, want invalid-localization identity", err)
			}
		})
	}
}

func TestProblemLocalizationAssetValidationRejectsMissingRequiredEnglishKey(t *testing.T) {
	t.Parallel()

	english := catalogWithoutKey(t, englishCatalogJSON, titleBadRequest)
	if _, err := loadLocalizer(english, persianCatalogJSON); !errors.Is(err, errInvalidLocalizationAssets) {
		t.Fatalf("loadLocalizer() error = %v, want invalid-localization identity", err)
	}
}

func TestProblemLocalizationAssetValidationAllowsMissingPersianTranslation(t *testing.T) {
	t.Parallel()

	persian := catalogWithoutKey(t, persianCatalogJSON, detailConflict)
	localizer, err := loadLocalizer(englishCatalogJSON, persian)
	if err != nil {
		t.Fatalf("loadLocalizer() error = %v", err)
	}
	if got, want := localizer.translate(localePersian, detailConflict, nil), "The request conflicts with current state."; got != want {
		t.Fatalf("fallback translation = %q, want %q", got, want)
	}
}

func TestProblemLocalizationAssetValidationRejectsIncompatibleParameters(t *testing.T) {
	t.Parallel()

	persian := catalogWithMessage(
		t,
		persianCatalogJSON,
		DetailInvalidUserRole,
		"نقش کاربری «{{.DifferentParameter}}» معتبر نیست.",
	)
	if _, err := loadLocalizer(englishCatalogJSON, persian); !errors.Is(err, errInvalidLocalizationAssets) {
		t.Fatalf("loadLocalizer() error = %v, want invalid-localization identity", err)
	}
}

func TestProblemLocalizationAssetValidationRejectsDuplicateAndEmptyKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		english []byte
	}{
		{
			name: "duplicate key",
			english: []byte(`{
				"title.bad_request": "first",
				"title.bad_request": "second"
			}`),
		},
		{
			name:    "empty key",
			english: []byte(`{"": "not addressable"}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := loadLocalizer(test.english, persianCatalogJSON); !errors.Is(err, errInvalidLocalizationAssets) {
				t.Fatalf("loadLocalizer() error = %v, want invalid-localization identity", err)
			}
		})
	}
}

func TestProblemLocalizationValidationErrorsDoNotExposeCatalogMessages(t *testing.T) {
	t.Parallel()

	const secret = "catalog-secret-value"
	english := []byte(`{"title.bad_request":{"secret":"` + secret + `"}}`)
	_, err := loadLocalizer(english, persianCatalogJSON)
	if !errors.Is(err, errInvalidLocalizationAssets) {
		t.Fatalf("loadLocalizer() error = %v, want invalid-localization identity", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("validation error exposed catalog contents: %v", err)
	}
}

func TestProblemLocalizationAssetLoaderDoesNotPanic(t *testing.T) {
	t.Parallel()

	assertSourceContainsNoPanicCall(t, "localization.go")
}

func TestTranslationKeyRegistryCoversEveryDefinedProblemKey(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), "localization.go", nil, 0)
	if err != nil {
		t.Fatalf("parse localization.go: %v", err)
	}
	registered := make(map[TranslationKey]struct{}, len(translationKeys))
	for _, key := range translationKeys {
		registered[key] = struct{}{}
	}

	defined := 0
	for _, declaration := range parsed.Decls {
		constants, ok := declaration.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, specification := range constants.Specs {
			values, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			keyType, ok := values.Type.(*ast.Ident)
			if !ok || keyType.Name != "TranslationKey" {
				continue
			}
			for _, value := range values.Values {
				literal, ok := value.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatalf("TranslationKey definition is not a string literal")
				}
				rawKey, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatalf("decode TranslationKey definition: %v", err)
				}
				key := TranslationKey(rawKey)
				defined++
				if _, exists := registered[key]; !exists {
					t.Errorf("defined problem translation key %q is not validated at bootstrap", key)
				}
			}
		}
	}
	if defined != len(registered) {
		t.Fatalf("defined TranslationKeys = %d, registered keys = %d", defined, len(registered))
	}
}

func catalogWithoutKey(t *testing.T, data []byte, key TranslationKey) []byte {
	t.Helper()

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatalf("decode test catalog: %v", err)
	}
	delete(messages, string(key))
	encoded, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("encode test catalog: %v", err)
	}
	return encoded
}

func catalogWithMessage(t *testing.T, data []byte, key TranslationKey, message string) []byte {
	t.Helper()

	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatalf("decode test catalog: %v", err)
	}
	messages[string(key)] = message
	encoded, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("encode test catalog: %v", err)
	}
	return encoded
}

func assertSourceContainsNoPanicCall(t *testing.T, path string) {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		identifier, ok := call.Fun.(*ast.Ident)
		if ok && identifier.Name == "panic" {
			t.Errorf("%s contains a production panic call", path)
		}
		return true
	})
}
