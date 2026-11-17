package blueprints

// Port Support/HasFieldBuilder.php: bentuk field di editor (Field), konversi dari/ke storage, dan validasi pohon field.
// Editor berjalan di Alpine (templates/layouts/field-editor.html); server menerima seluruh pohon sebagai JSON saat save.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"draftcms-lite/internal/web"
)

type Option struct{ Value, Label string }

var Fieldtypes = []Option{{"text", "Text"}, {"textarea", "Textarea"}, {"markdown", "Markdown"}, {"bard", "Bard (rich text)"}, {"number", "Number"}, {"toggle", "Toggle"}, {"date", "Date"},
	{"select", "Select"}, {"radio", "Radio"}, {"checkbox", "Checkbox"}, {"replicator", "Replicator"}, {"assets", "Assets"}, {"link", "Link"}, {"fieldset", "Fieldset"}, {"taxonomy", "Taxonomy"}, {"author", "Author"}}
var ConditionOperators = []Option{{"equals", "equals"}, {"not", "not"}, {"contains", "contains"}, {"contains_any", "contains any"}, {"strict", "==="}}
var BardToolbarButtons = []Option{{"bold", "Bold"}, {"italic", "Italic"}, {"underline", "Underline"}, {"strike", "Strikethrough"}, {"h1", "Heading 1"}, {"h2", "Heading 2"}, {"h3", "Heading 3"},
	{"h4", "Heading 4"}, {"h5", "Heading 5"}, {"h6", "Heading 6"}, {"bulletlist", "Bulleted list"}, {"orderedlist", "Numbered list"}, {"blockquote", "Blockquote"}, {"link", "Link"}, {"image", "Image"}}

var optionBasedTypes = []string{"select", "radio", "checkbox"}
var noPlaceholderTypes = []string{"toggle", "date", "replicator", "assets", "fieldset", "taxonomy", "author"}
var noDefaultTypes = []string{"replicator", "assets", "link", "fieldset", "taxonomy", "author"}
var itemsTypes = []string{"assets", "checkbox", "taxonomy", "replicator"}
var conditionModes = []string{"always", "show", "hide"}

var handleRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var validationRe = regexp.MustCompile(`^[a-zA-Z0-9_:,|.\-]*$`)
var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// laravelRules = pengganti ruleStringError(): source menjalankan Validator Laravel untuk mengecek nama rule; di sini cukup daftar nama rule bawaan.
var laravelRules = strings.Fields(`accepted accepted_if active_url after after_or_equal alpha alpha_dash alpha_num array ascii bail before before_or_equal between boolean confirmed contains current_password date date_equals date_format decimal declined declined_if different digits digits_between dimensions distinct doesnt_start_with doesnt_end_with email ends_with enum exclude exclude_if exclude_unless exclude_with exclude_without exists extensions file filled gt gte hex_color image in in_array integer ip ipv4 ipv6 json list lt lte lowercase mac_address max max_digits mimetypes mimes min min_digits missing missing_if missing_unless missing_with missing_with_all multiple_of not_in not_regex nullable numeric present present_if present_unless present_with present_with_all prohibited prohibited_if prohibited_unless prohibits regex required required_if required_if_accepted required_if_declined required_unless required_with required_with_all required_without required_without_all required_array_keys same size sometimes starts_with string timezone unique uppercase url ulid uuid`)

// Str: nilai editor selalu string, tapi JSON dari Alpine bisa mengirim angka/bool (input number, data lama).
type Str string

func (s *Str) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = Str(anyToString(v))
	return nil
}

func anyToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		parts := make([]string, len(t))
		for i, p := range t {
			parts[i] = anyToString(p)
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprint(v)
}

// Field = satu field dalam bentuk editor (mapFieldFromStorage / fieldDefaults).
type Field struct {
	UID                string   `json:"_uid"`
	Handle             string   `json:"handle"`
	Display            string   `json:"display"`
	Type               string   `json:"type"`
	Required           bool     `json:"required"`
	Localizable        bool     `json:"localizable"`
	Options            Str      `json:"options"`
	Placeholder        Str      `json:"placeholder"`
	Default            Str      `json:"default"`
	Validation         Str      `json:"validation"`
	Instructions       Str      `json:"instructions"`
	Sometimes          bool     `json:"sometimes"`
	ConditionMode      string   `json:"condition_mode"`
	ConditionField     Str      `json:"condition_field"`
	ConditionOperator  string   `json:"condition_operator"`
	ConditionValue     Str      `json:"condition_value"`
	MinItems           Str      `json:"min_items"`
	MaxItems           Str      `json:"max_items"`
	CollapsedByDefault bool     `json:"collapsed_by_default"`
	Toolbar            []string `json:"toolbar"`
	Tab                string   `json:"tab"`
	Section            string   `json:"section"`
	ReplicatorFields   []Field  `json:"replicator_fields"`
}

func FieldDefaults() Field {
	return Field{UID: uid(), Type: "text", ConditionMode: "always", ConditionOperator: "equals", Toolbar: []string{}, Tab: "main", ReplicatorFields: []Field{}}
}

func uid() string { return web.Token()[:12] }

// NormalizeTree = normalizeFieldsTree(): isi key yang hilang dengan default (nil slice → []), rekursif.
func NormalizeTree(fields []Field) []Field {
	out := make([]Field, len(fields))
	for i, f := range fields {
		if f.UID == "" {
			f.UID = uid()
		}
		if f.Type == "" {
			f.Type = "text"
		}
		if f.ConditionMode == "" {
			f.ConditionMode = "always"
		}
		if f.ConditionOperator == "" {
			f.ConditionOperator = "equals"
		}
		if f.Tab == "" {
			f.Tab = "main"
		}
		if f.Toolbar == nil {
			f.Toolbar = []string{}
		}
		f.ReplicatorFields = NormalizeTree(f.ReplicatorFields)
		out[i] = f
	}
	return out
}

// FromStorage = mapFieldFromStorage().
func FromStorage(raw []any) []Field {
	out := []Field{}
	for _, r := range raw {
		m, _ := r.(map[string]any)
		if m == nil {
			continue
		}
		f := FieldDefaults()
		str := func(k string) string { return anyToString(m[k]) }
		boolean := func(k string) bool { b, _ := m[k].(bool); return b || m[k] == float64(1) }
		f.Handle, f.Display = str("handle"), str("display")
		if t := str("type"); t != "" {
			f.Type = t
		}
		f.Required, f.Localizable, f.Sometimes, f.CollapsedByDefault = boolean("required"), boolean("localizable"), boolean("validate_sometimes"), boolean("collapsed_by_default")
		f.Options, f.Placeholder, f.Validation, f.Instructions = Str(str("options")), Str(str("placeholder")), Str(str("validation")), Str(str("instructions"))
		if f.Type == "toggle" { // storage boolean (atau string "true"/"false" lama)
			f.Default = Str(anyToString(m["default"] == true || m["default"] == "true"))
		} else {
			f.Default = Str(str("default"))
		}
		if c, ok := m["conditions"].(map[string]any); ok {
			f.ConditionMode, f.ConditionField, f.ConditionOperator, f.ConditionValue = anyToString(c["mode"]), Str(anyToString(c["field"])), anyToString(c["operator"]), Str(anyToString(c["value"]))
			if f.ConditionMode == "" {
				f.ConditionMode = "always"
			}
			if f.ConditionOperator == "" {
				f.ConditionOperator = "equals"
			}
		}
		if m["min_items"] != nil {
			f.MinItems = Str(str("min_items"))
		}
		if m["max_items"] != nil {
			f.MaxItems = Str(str("max_items"))
		}
		if tb, ok := m["toolbar"].([]any); ok {
			for _, b := range tb {
				f.Toolbar = append(f.Toolbar, anyToString(b))
			}
		}
		if t := str("tab"); t != "" {
			f.Tab = t
		}
		f.Section = str("section")
		if sub, ok := m["replicator_fields"].([]any); ok {
			f.ReplicatorFields = FromStorage(sub)
		}
		out = append(out, f)
	}
	return out
}

// Headline = Str::headline(): "hero_title" → "Hero Title".
func Headline(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func csv(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func intOrNil(s string) any {
	if s == "" {
		return nil
	}
	n, _ := strconv.Atoi(s)
	return n
}

// ToStorage = normalizeFieldForStorage().
func ToStorage(f Field, topLevel bool) map[string]any {
	display := f.Display
	if display == "" {
		display = Headline(f.Handle)
	}
	n := map[string]any{
		"handle": f.Handle, "display": display, "type": f.Type, "required": f.Required, "localizable": f.Localizable,
		"validation": nullable(string(f.Validation)), "instructions": nullable(string(f.Instructions)), "validate_sometimes": f.Sometimes,
	}
	if slices.Contains(noPlaceholderTypes, f.Type) {
		n["placeholder"] = nil
	} else {
		n["placeholder"] = nullable(string(f.Placeholder))
	}
	switch {
	case slices.Contains(noDefaultTypes, f.Type):
		n["default"] = nil
	case f.Type == "toggle":
		n["default"] = f.Default == "true"
	default:
		n["default"] = nullable(string(f.Default))
	}
	if f.ConditionMode != "always" {
		n["conditions"] = map[string]any{"mode": f.ConditionMode, "field": string(f.ConditionField), "operator": f.ConditionOperator, "value": string(f.ConditionValue)}
	} else {
		n["conditions"] = nil
	}
	if slices.Contains(optionBasedTypes, f.Type) {
		n["options"] = csv(string(f.Options))
	}
	if f.Type == "checkbox" {
		n["default"] = csv(string(f.Default))
	}
	if f.Type == "assets" || f.Type == "fieldset" || f.Type == "taxonomy" {
		n["options"] = nullable(strings.TrimSpace(string(f.Options)))
	}
	if f.Type == "link" {
		n["options"] = csv(string(f.Options))
	}
	if slices.Contains(itemsTypes, f.Type) {
		n["min_items"], n["max_items"] = intOrNil(string(f.MinItems)), intOrNil(string(f.MaxItems))
	}
	if f.Type == "bard" {
		n["toolbar"] = f.Toolbar
	}
	if f.Type == "replicator" {
		n["collapsed_by_default"] = f.CollapsedByDefault
		subs := make([]map[string]any, len(f.ReplicatorFields))
		for i, s := range f.ReplicatorFields {
			subs[i] = ToStorage(s, false)
		}
		n["replicator_fields"] = subs
	}
	if topLevel {
		n["tab"] = f.Tab
		n["section"] = nullable(f.Section)
	}
	return n
}

// Lookups = daftar handle yang boleh dipakai field assets/link/fieldset/taxonomy (availableAssetContainers dst.).
type Lookups struct{ AssetContainers, Collections, Fieldsets, Taxonomies []string }

func values(opts []Option) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Value
	}
	return out
}

// FieldsetHandlesInUse = fieldsetHandlesInUse(): satu entri per pemakaian, rekursif.
func FieldsetHandlesInUse(fields []Field) (out []string) {
	for _, f := range fields {
		if f.Type == "fieldset" && f.Options != "" {
			out = append(out, string(f.Options))
		}
		out = append(out, FieldsetHandlesInUse(f.ReplicatorFields)...)
	}
	return
}

func ruleStringError(rules string) string {
	if rules == "" {
		return ""
	}
	for _, rule := range strings.Split(rules, "|") {
		name, _, _ := strings.Cut(rule, ":")
		if name = strings.TrimSpace(name); name != "" && !slices.Contains(laravelRules, name) {
			return "Unknown or invalid Laravel validation rule."
		}
	}
	return ""
}

// ValidateTree = validateFieldsTree(): error ber-key "fields.0.handle", "fields.0.replicator_fields.1.options", dst.
// tabHandles kosong = fieldset (tanpa cek tab/section). allFields = seluruh pohon untuk cek fieldset dipakai dua kali.
func ValidateTree(fields []Field, prefix string, tabHandles []string, sectionsByTab map[string][]string, lk Lookups, allFields []Field) map[string]string {
	errs := map[string]string{}
	handles := []string{}
	for _, f := range fields {
		if f.Type != "fieldset" {
			handles = append(handles, f.Handle)
		}
	}
	seen := map[string]bool{}
	for _, h := range handles {
		if seen[h] {
			errs[prefix] = "Field handles must be unique within this level."
		}
		seen[h] = true
	}
	for i, f := range fields {
		k := func(attr string) string { return fmt.Sprintf("%s.%d.%s", prefix, i, attr) }
		set := func(attr, msg string) {
			if _, ok := errs[k(attr)]; !ok {
				errs[k(attr)] = msg
			}
		}
		// fieldValidationRules()
		if f.Type != "fieldset" {
			switch {
			case f.Handle == "":
				set("handle", "The handle field is required.")
			case len(f.Handle) > 255:
				set("handle", "The handle field must not be greater than 255 characters.")
			case !handleRe.MatchString(f.Handle):
				set("handle", "Field handle must be lowercase letters, numbers and underscores, starting with a letter.")
			case f.Handle == "title":
				set("handle", `"title" is a reserved handle — it is already a default field on every entry.`)
			}
			if f.Display == "" {
				set("display", "The label field is required.")
			}
		}
		if len(f.Display) > 255 {
			set("display", "The label field must not be greater than 255 characters.")
		}
		if !slices.Contains(values(Fieldtypes), f.Type) {
			set("type", "The selected type is invalid.")
		}
		for attr, v := range map[string]string{"placeholder": string(f.Placeholder), "default": string(f.Default), "validation": string(f.Validation), "condition_field": string(f.ConditionField), "condition_value": string(f.ConditionValue)} {
			if len(v) > 255 {
				set(attr, "The "+strings.ReplaceAll(attr, "_", " ")+" field must not be greater than 255 characters.")
			}
		}
		if len(f.Instructions) > 1000 {
			set("instructions", "The instructions field must not be greater than 1000 characters.")
		}
		if !validationRe.MatchString(string(f.Validation)) {
			set("validation", "Validation rules may only contain letters, numbers and : , | . _ -")
		}
		if !slices.Contains(conditionModes, f.ConditionMode) {
			set("condition_mode", "The selected condition mode is invalid.")
		}
		if f.ConditionOperator != "" && !slices.Contains(values(ConditionOperators), f.ConditionOperator) {
			set("condition_operator", "The selected condition operator is invalid.")
		}
		for attr, v := range map[string]string{"min_items": string(f.MinItems), "max_items": string(f.MaxItems)} {
			if v != "" {
				if n, err := strconv.Atoi(v); err != nil || n < 0 {
					set(attr, "The "+strings.ReplaceAll(attr, "_", " ")+" field must be at least 0.")
				}
			}
		}
		for _, b := range f.Toolbar {
			if !slices.Contains(values(BardToolbarButtons), b) {
				set("toolbar", "The selected toolbar button is invalid.")
			}
		}
		if f.Default != "" && f.Type == "date" && !dateRe.MatchString(string(f.Default)) {
			set("default", "Default value must be a valid date (YYYY-MM-DD).")
		}
		if msg := ruleStringError(string(f.Validation)); msg != "" {
			set("validation", msg)
		}
		// conditionErrors()
		if _, bad := errs[k("condition_mode")]; !bad && f.ConditionMode != "always" {
			switch {
			case f.ConditionField == "":
				set("condition_field", "Select a field to base this condition on.")
			case string(f.ConditionField) == f.Handle:
				set("condition_field", "A field cannot depend on its own condition.")
			case !slices.Contains(handles, string(f.ConditionField)):
				set("condition_field", "That field does not exist in this scope.")
			}
			if f.ConditionOperator == "" {
				set("condition_operator", "Select an operator for this condition.")
			}
			if f.ConditionValue == "" {
				set("condition_value", "Enter a value to compare against.")
			}
		}
		opt := strings.TrimSpace(string(f.Options))
		switch f.Type {
		case "assets":
			if opt == "" {
				set("options", "Select an asset container.")
			} else if !slices.Contains(lk.AssetContainers, opt) {
				set("options", "That asset container does not exist.")
			}
		case "taxonomy":
			if opt == "" {
				set("options", "Select a taxonomy.")
			} else if !slices.Contains(lk.Taxonomies, opt) {
				set("options", "That taxonomy does not exist.")
			}
		case "fieldset":
			if opt == "" {
				set("options", "Select a fieldset.")
			} else if !slices.Contains(lk.Fieldsets, opt) {
				set("options", "That fieldset does not exist.")
			} else if n := 0; func() bool {
				for _, h := range FieldsetHandlesInUse(allFields) {
					if h == opt {
						n++
					}
				}
				return n > 1
			}() {
				set("options", "That fieldset is already used in this blueprint.")
			}
		case "link":
			for _, h := range csv(string(f.Options)) {
				if !slices.Contains(lk.Collections, h) {
					set("options", fmt.Sprintf(`"%s" is not a valid collection.`, h))
					break
				}
			}
		}
		if len(tabHandles) > 0 {
			if !slices.Contains(tabHandles, f.Tab) {
				set("tab", "That tab does not exist.")
			} else if f.Section != "" && !slices.Contains(sectionsByTab[f.Tab], f.Section) {
				set("section", "That section does not exist in this tab.")
			}
		}
		if slices.Contains(itemsTypes, f.Type) && f.MinItems != "" && f.MaxItems != "" {
			if _, bad := errs[k("max_items")]; !bad {
				mn, _ := strconv.Atoi(string(f.MinItems))
				mx, _ := strconv.Atoi(string(f.MaxItems))
				if mx < mn {
					set("max_items", "Maximum selections must be greater than or equal to minimum selections.")
				}
			}
		}
		if f.Type == "replicator" {
			if len(f.ReplicatorFields) == 0 {
				set("replicator_fields", "Add at least one field to the replicator.")
			} else {
				for key, msg := range ValidateTree(f.ReplicatorFields, k("replicator_fields"), nil, nil, lk, allFields) {
					errs[key] = msg
				}
			}
		}
	}
	return errs
}
