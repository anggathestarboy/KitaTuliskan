package navigation

// Mode blueprint NavigationTreeEditor (navigation-content-editor.blade.php): form dari field blueprint navigation, nilainya
// disimpan sebagai objek di tabel trees (per locale). Berisi: blueprint efektif, load/simpan data + override per bahasa,
// kondisi tampil/sembunyi field, dan validasi (fieldRulesFor).

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"draftcms-lite/internal/web"
)

type bp struct {
	Handle string           `json:"handle"`
	Tabs   []any            `json:"tabs"`
	Fields []map[string]any `json:"fields"`
}

// BP / Site = tipe yang dipakai Source; dialiaskan supaya modul lain (Globals) bisa menyebutnya.
type (
	BP   = bp
	Site = site
)

// BlueprintFor = effectiveBlueprint(): pilihan eksplisit, kalau kosong blueprint ber-namespace "{ns}.{handle}". nil bila tidak ada.
func BlueprintFor(a *web.App, ns, handle, explicit string) *bp {
	q, arg := `SELECT handle,data FROM blueprints WHERE namespace=? ORDER BY id LIMIT 1`, ns+"."+handle
	if explicit != "" {
		q, arg = `SELECT handle,data FROM blueprints WHERE handle=? ORDER BY id LIMIT 1`, explicit
	}
	var b bp
	var raw []byte
	if a.DB.QueryRow(q, arg).Scan(&b.Handle, &raw) != nil {
		return nil
	}
	var d struct {
		Tabs   []any
		Fields []map[string]any
	}
	json.Unmarshal(raw, &d)
	b.Tabs, b.Fields = d.Tabs, d.Fields
	if b.Tabs == nil {
		b.Tabs = []any{map[string]any{"handle": "main", "display": "Main"}}
	}
	if b.Fields == nil {
		b.Fields = []map[string]any{}
	}
	return &b
}

func blueprintOf(a *web.App, n navRow) *bp {
	var s struct{ Blueprint string }
	json.Unmarshal(n.settings, &s)
	return BlueprintFor(a, "navigations", n.Handle, s.Blueprint)
}

// BlueprintOptions = availableBlueprints(): untuk select Blueprint di modal.
func BlueprintOptions(a *web.App) []map[string]string {
	out := []map[string]string{}
	rows, err := a.DB.Query(`SELECT namespace,handle FROM blueprints ORDER BY namespace, handle`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var ns sql.NullString
		var h string
		rows.Scan(&ns, &h)
		label := h
		if ns.String != "" {
			label = ns.String + "." + h
		}
		out = append(out, map[string]string{"handle": h, "label": label})
	}
	return out
}

// CreateBlueprint = Blueprint::firstOrCreate: blueprint kosong "{ns}.{handle}" (ns: "navigations" / "globals"); true bila baru dibuat.
func CreateBlueprint(a *web.App, ns, handle string) (bool, error) {
	var id int64
	err := a.DB.QueryRow(`SELECT id FROM blueprints WHERE namespace=? AND handle=?`, ns+"."+handle, handle).Scan(&id)
	if err != sql.ErrNoRows {
		return false, err
	}
	now := web.Now()
	_, err = a.DB.Exec(`INSERT INTO blueprints(namespace,handle,data,created_at,updated_at) VALUES(?,?,?,?,?)`,
		ns+"."+handle, handle, `{"tabs":[{"handle":"main","display":"Main","sections":[]}],"fields":[]}`, now, now)
	return err == nil, err
}

/* ---------- helper nilai & field ---------- */

// str = (string) ala PHP untuk nilai JSON.
func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return ""
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func subfields(f map[string]any) (out []map[string]any) {
	raw, _ := f["replicator_fields"].([]any)
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return
}

func truthy(v any) bool { b, _ := v.(bool); return b }

func intOf(v any) (int, bool) {
	f, ok := v.(float64)
	return int(f), ok
}

func optionList(f map[string]any) (out []string) {
	raw, _ := f["options"].([]any)
	for _, o := range raw {
		out = append(out, str(o))
	}
	return
}

/* ---------- kondisi tampil (EvaluatesFieldConditions) ---------- */

func visible(f map[string]any, scope map[string]any) bool {
	c, _ := f["conditions"].(map[string]any)
	mode := str(c["mode"])
	if c == nil || mode == "" || mode == "always" {
		return true
	}
	m := condMatches(c, scope[str(c["field"])])
	if mode == "show" {
		return m
	}
	return !m
}

func condMatches(c map[string]any, actual any) bool {
	expected := str(c["value"])
	switch str(c["operator"]) {
	case "not":
		return !condEquals(actual, expected)
	case "strict":
		s, ok := actual.(string)
		return ok && s == expected
	case "contains":
		if arr, ok := actual.([]any); ok {
			return slices.ContainsFunc(arr, func(x any) bool { return str(x) == expected })
		}
		return strings.Contains(str(actual), expected)
	case "contains_any":
		arr, ok := actual.([]any)
		if !ok {
			return false
		}
		for _, w := range strings.Split(expected, ",") {
			if w = strings.TrimSpace(w); w != "" && slices.ContainsFunc(arr, func(x any) bool { return str(x) == w }) {
				return true
			}
		}
		return false
	}
	return condEquals(actual, expected)
}

func condEquals(actual any, expected string) bool {
	switch t := actual.(type) {
	case bool:
		return t == slices.Contains([]string{"1", "true", "on", "yes"}, strings.ToLower(strings.TrimSpace(expected)))
	case []any:
		return slices.ContainsFunc(t, func(x any) bool { return str(x) == expected })
	}
	return str(actual) == expected
}

/* ---------- load ---------- */

type site struct {
	Handle string `json:"handle"`
	Name   string `json:"name"`
}

// Sites = semua site menurut urutan; dipakai modul lain untuk daftar "Languages".
func Sites(a *web.App) []site {
	out := []site{}
	rows, err := a.DB.Query("SELECT handle,name FROM sites ORDER BY `order`")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var s site
		rows.Scan(&s.Handle, &s.Name)
		out = append(out, s)
	}
	return out
}

// resolveSite: bahasa yang sedang diedit. Tanpa site → satu baris tanpa locale yang selalu "origin";
// selain itu locale = handle site, origin = site teratas.
func resolveSite(ss []site, req string) (active, origin string, isOrigin bool) {
	if len(ss) == 0 {
		return "", "", true
	}
	origin, active = ss[0].Handle, ss[0].Handle
	for _, s := range ss {
		if s.Handle == req {
			active = req
		}
	}
	return active, origin, active == origin
}

// fill = fillFieldsData: bentuk data sesuai pohon field (replicator → baris, assets/taxonomy → array, lainnya default).
func fill(fields []map[string]any, existing map[string]any) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		h, t := str(f["handle"]), str(f["type"])
		switch t {
		case "replicator":
			rows := []any{}
			src, _ := existing[h].([]any)
			for _, r := range src {
				rm, _ := r.(map[string]any)
				row := fill(subfields(f), rm)
				row["_enabled"] = true // baris replicator bisa dimatikan; bawaannya aktif
				if en, ok := rm["_enabled"]; ok {
					row["_enabled"] = en
				}
				rows = append(rows, row)
			}
			out[h] = rows
		case "assets", "taxonomy":
			if v, ok := existing[h].([]any); ok {
				out[h] = v
			} else {
				out[h] = []any{}
			}
		default:
			v := existing[h]
			if v == nil {
				v = f["default"]
			}
			if v == nil {
				if t == "toggle" {
					v = false
				} else {
					v = ""
				}
			}
			out[h] = v
		}
	}
	return out
}

// entryForSite = Entry::entryForSite: saudara terjemahan entry di site tsb (origin_id yang sama).
func entryForSite(a *web.App, id, site string) string {
	var s string
	var origin sql.NullString
	if a.DB.QueryRow(`SELECT site,origin_id FROM entries WHERE id=?`, id).Scan(&s, &origin) != nil {
		return ""
	}
	if s == site {
		return id
	}
	o := origin.String
	if o == "" {
		o = id
	}
	var out string
	a.DB.QueryRow(`SELECT id FROM entries WHERE origin_id=? AND site=? LIMIT 1`, o, site).Scan(&out)
	return out
}

// localizeLinks = Entry::localizeLinkValues: "entry::{id}" diarahkan ke terjemahannya di $site (kalau ada).
func localizeLinks(a *web.App, data map[string]any, fields []map[string]any, site string) {
	for _, f := range fields {
		h := str(f["handle"])
		switch str(f["type"]) {
		case "replicator":
			rows, _ := data[h].([]any)
			for _, r := range rows {
				if rm, ok := r.(map[string]any); ok {
					localizeLinks(a, rm, subfields(f), site)
				}
			}
		case "link":
			if v, ok := data[h].(string); ok && strings.HasPrefix(v, "entry::") {
				if id := entryForSite(a, v[7:], site); id != "" {
					data[h] = "entry::" + id
				}
			}
		}
	}
}

func nonLocalizable(fields []map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, f := range fields {
		if !truthy(f["localizable"]) {
			out[str(f["handle"])] = true
		}
	}
	return out
}

// loadContent = loadBlueprintData: baris locale non-origin hanya memuat field yang dioverride; sisanya mengikuti bahasa default.
func loadContent(a *web.App, src Source, handle string, fields []map[string]any, active, origin string) map[string]any {
	own := src.Load(handle, active)
	if active == origin {
		return fill(fields, own)
	}
	merged := src.Load(handle, origin)
	skip := nonLocalizable(fields)
	for k, v := range own {
		if !skip[k] {
			merged[k] = v
		}
	}
	data := fill(fields, merged)
	localizeLinks(a, data, fields, active)
	return data
}

// eq = perbandingan longgar ala PHP == untuk menentukan field yang benar-benar dioverride.
func eq(x, y any) bool {
	switch xv := x.(type) {
	case []any:
		yv, ok := y.([]any)
		if !ok || len(xv) != len(yv) {
			return false
		}
		for i := range xv {
			if !eq(xv[i], yv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		yv, ok := y.(map[string]any)
		if !ok || len(xv) != len(yv) {
			return false
		}
		for k, v := range xv {
			if w, ok := yv[k]; !ok || !eq(v, w) {
				return false
			}
		}
		return true
	}
	_, ya := y.([]any)
	_, ym := y.(map[string]any)
	return !ya && !ym && str(x) == str(y)
}

/* ---------- validasi (fieldRulesFor) ---------- */

type chk struct {
	a     *web.App
	site  string
	errs  map[string]string
	first string
}

func (c *chk) fail(key, msg string) {
	if _, dup := c.errs[key]; !dup {
		c.errs[key] = msg
		if c.first == "" {
			c.first = msg
		}
	}
}

func empty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	}
	return false
}

func count(a *web.App, q string, args ...any) (n int) {
	a.DB.QueryRow(q, args...).Scan(&n)
	return
}

func isInt(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), t == math.Trunc(t)
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n, err == nil
	}
	return 0, false
}

func isNumeric(v any) bool {
	switch t := v.(type) {
	case float64:
		return true
	case string:
		_, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return err == nil
	}
	return false
}

func isBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return true
	case float64:
		return t == 0 || t == 1
	case string:
		return slices.Contains([]string{"0", "1", "true", "false"}, t)
	}
	return false
}

func isDate(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, l := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02T15:04:05Z07:00"} {
		if _, err := time.Parse(l, strings.TrimSpace(s)); err == nil {
			return true
		}
	}
	return false
}

var alphaRe = map[string]func(rune) bool{
	"alpha":      unicode.IsLetter,
	"alpha_num":  func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) },
	"alpha_dash": func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' },
}
var alphaMsg = map[string]string{"alpha": "letters", "alpha_num": "letters and numbers", "alpha_dash": "letters, numbers, dashes, and underscores"}

// ruleCheck: subset rule Laravel yang umum dipakai di "validation" blueprint (min, max, email, url, numeric, integer, alpha*).
// Rule lain diterima editor blueprint tapi tidak dievaluasi di sini.
func ruleCheck(rules string, v any, label string) string {
	list := strings.Split(rules, "|")
	numeric := slices.Contains(list, "numeric") || slices.Contains(list, "integer")
	s := str(v)
	for _, r := range list {
		name, arg, _ := strings.Cut(strings.TrimSpace(r), ":")
		n, _ := strconv.ParseFloat(arg, 64)
		size := float64(utf8.RuneCountInString(s))
		unit := " characters"
		if numeric {
			size, _ = strconv.ParseFloat(strings.TrimSpace(s), 64)
			unit = ""
		}
		switch name {
		case "min":
			if size < n {
				return fmt.Sprintf("The %s field must be at least %s%s.", label, arg, unit)
			}
		case "max":
			if size > n {
				return fmt.Sprintf("The %s field must not be greater than %s%s.", label, arg, unit)
			}
		case "email":
			if _, err := mail.ParseAddress(s); err != nil || strings.ContainsAny(s, " <>") {
				return fmt.Sprintf("The %s field must be a valid email address.", label)
			}
		case "url":
			if u, err := url.ParseRequestURI(s); err != nil || u.Scheme == "" || u.Host == "" {
				return fmt.Sprintf("The %s field must be a valid URL.", label)
			}
		case "numeric":
			if !isNumeric(v) {
				return fmt.Sprintf("The %s field must be a number.", label)
			}
		case "integer":
			if _, ok := isInt(v); !ok {
				return fmt.Sprintf("The %s field must be an integer.", label)
			}
		case "alpha", "alpha_num", "alpha_dash":
			if strings.IndexFunc(s, func(r rune) bool { return !alphaRe[name](r) }) >= 0 {
				return fmt.Sprintf("The %s field must only contain %s.", label, alphaMsg[name])
			}
		}
	}
	return ""
}

// check memvalidasi $vals terhadap $fields dan mengembalikan nilai bersih (hanya field blueprint; field tersembunyi
// kondisi dibiarkan apa adanya seperti rule 'nullable' di source). Kunci error = path ("links.0.title").
func (c *chk) check(fields []map[string]any, vals map[string]any, prefix string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		h, typ := str(f["handle"]), str(f["type"])
		key := prefix + h
		v, present := vals[h]
		label := str(f["display"])
		if label == "" {
			label = h
		}
		if !present && truthy(f["validate_sometimes"]) {
			continue
		}
		keep := func(x any) {
			if present {
				out[h] = x
			}
		}
		req := truthy(f["required"])
		required := fmt.Sprintf("The %s field is required.", label)

		minItems, hasMin := intOf(f["min_items"])
		maxItems, hasMax := intOf(f["max_items"])

		if typ == "replicator" {
			rows, isArr := v.([]any)
			switch {
			case v != nil && !isArr:
				c.fail(key, fmt.Sprintf("The %s field must be an array.", label))
			case !visible(f, vals):
				keep(v)
			case req && len(rows) == 0:
				c.fail(key, required)
			case hasMin && len(rows) < minItems:
				c.fail(key, fmt.Sprintf("The %s field must have at least %d items.", label, minItems))
			case hasMax && len(rows) > maxItems:
				c.fail(key, fmt.Sprintf("The %s field must not have more than %d items.", label, maxItems))
			default:
				cleaned := []any{}
				for i, r := range rows {
					rm, ok := r.(map[string]any)
					if !ok {
						c.fail(fmt.Sprintf("%s.%d", key, i), fmt.Sprintf("The %s field must be an array.", label))
						continue
					}
					row := c.check(subfields(f), rm, fmt.Sprintf("%s.%d.", key, i))
					en, ok := rm["_enabled"]
					if !ok || !isBool(en) {
						en = true
					}
					row["_enabled"] = en
					cleaned = append(cleaned, row)
				}
				keep(cleaned)
			}
			continue
		}
		if !visible(f, vals) {
			keep(v)
			continue
		}
		switch typ {
		case "assets", "taxonomy", "checkbox":
			if v == nil {
				if req {
					c.fail(key, required)
				}
				keep(v)
				continue
			}
			arr, ok := v.([]any)
			if !ok {
				c.fail(key, fmt.Sprintf("The %s field must be an array.", label))
				continue
			}
			clean := []any{}
			for _, x := range arr {
				if typ != "checkbox" && empty(x) { // select tunggal taxonomy yang dikosongkan mengirim [""]
					continue
				}
				clean = append(clean, x)
			}
			switch {
			case req && len(clean) == 0:
				c.fail(key, required)
			case hasMin && len(clean) < minItems:
				c.fail(key, fmt.Sprintf("The %s field must have at least %d items.", label, minItems))
			case hasMax && len(clean) > maxItems:
				c.fail(key, fmt.Sprintf("The %s field must not have more than %d items.", label, maxItems))
			default:
				ids := []any{}
				for _, x := range clean {
					if typ == "checkbox" {
						if !slices.Contains(optionList(f), str(x)) {
							c.fail(key, fmt.Sprintf("The selected %s is invalid.", label))
						}
						ids = append(ids, x)
						continue
					}
					id, ok := isInt(x)
					if ok && typ == "assets" {
						ok = count(c.a, `SELECT COUNT(*) FROM assets_meta WHERE id=? AND container=?`, id, str(f["options"])) > 0
					} else if ok {
						q, args := `SELECT COUNT(*) FROM taxonomy_terms WHERE id=? AND taxonomy=?`, []any{id, str(f["options"])}
						if c.site != "" {
							q, args = q+` AND site=?`, append(args, c.site)
						}
						ok = count(c.a, q, args...) > 0
					}
					if !ok {
						c.fail(key, fmt.Sprintf("The selected %s is invalid.", label))
					}
					if typ == "checkbox" {
						ids = append(ids, x)
					} else {
						ids = append(ids, id)
					}
				}
				keep(ids)
			}
			continue
		}

		// Nilai kosong: hanya 'required' yang berlaku (rule lain dilewati Laravel untuk string kosong).
		if empty(v) {
			if req {
				c.fail(key, required)
			}
			keep(v)
			continue
		}
		msg := ""
		switch typ {
		case "link":
			s, ok := v.(string)
			switch {
			case !ok:
				msg = fmt.Sprintf("The %s field must be a string.", label)
			case utf8.RuneCountInString(s) > 2048:
				msg = fmt.Sprintf("The %s field must not be greater than 2048 characters.", label)
			case !strings.HasPrefix(s, "entry::") && str(f["validation"]) != "":
				msg = ruleCheck(str(f["validation"]), v, label)
			}
		case "number":
			if !isNumeric(v) {
				msg = fmt.Sprintf("The %s field must be a number.", label)
			}
		case "toggle":
			if !isBool(v) {
				msg = fmt.Sprintf("The %s field must be true or false.", label)
			}
		case "date":
			if !isDate(v) {
				msg = fmt.Sprintf("The %s field must be a valid date.", label)
			}
		case "select", "radio":
			if !slices.Contains(optionList(f), str(v)) {
				msg = fmt.Sprintf("The selected %s is invalid.", label)
			}
		default:
			if _, ok := v.(string); !ok {
				msg = fmt.Sprintf("The %s field must be a string.", label)
			}
		}
		if msg == "" && typ != "link" && str(f["validation"]) != "" {
			msg = ruleCheck(str(f["validation"]), v, label)
		}
		if msg != "" {
			c.fail(key, msg)
			continue
		}
		keep(v)
	}
	return out
}

/* ---------- route ---------- */

// Source = apa yang membedakan satu modul dari yang lain untuk form nilai berbasis blueprint (Navigation, Globals).
type Source struct {
	Find             func(handle string) (title string, b *bp, err error) // b == nil: belum ada blueprint
	CanView, CanEdit func(u *web.User, handle string) bool
	Sites            func(handle string) []site                 // site yang dipakai item ini; kosong = satu baris tanpa locale
	Load             func(handle, locale string) map[string]any // locale "" = NULL
	Save             func(handle, locale string, data map[string]any) error
	EmptyOK          bool                               // true: blueprint tanpa field tetap dibuka (Globals menampilkan keadaan kosong)
	Extra            func(handle string) map[string]any // ditambahkan ke respons GET (mis. saveAction)
	FlashOnSave      func(handle string) string         // non-kosong: pesan flash yang tampil di halaman berikutnya
}

// RegisterContent memasang GET/POST {prefix}/api/content/{handle}.
func RegisterContent(mux *http.ServeMux, a *web.App, prefix string, src Source) {
	// resolve: item + blueprint + izin; dipakai GET dan POST.
	resolve := func(r *http.Request, write bool) (handle, title string, b *bp, err error) {
		u := web.CurrentUser(r)
		handle = r.PathValue("handle")
		if title, b, err = src.Find(handle); err != nil {
			return
		}
		if write && !src.CanEdit(u, handle) || !write && !src.CanView(u, handle) {
			err = errForbidden
		}
		if err == nil && (b == nil || len(b.Fields) == 0) {
			if !src.EmptyOK {
				err = apiErr("This navigation has no blueprint yet.")
			} else if b == nil {
				b = &bp{Tabs: []any{map[string]any{"handle": "main", "display": "Main"}}, Fields: []map[string]any{}}
			}
		}
		return
	}
	labelled := func(q string) []map[string]string { // id, group, site, slug, data → {id, group, site, label}
		out := []map[string]string{}
		rows, err := a.DB.Query(q)
		if err != nil {
			return out
		}
		defer rows.Close()
		for rows.Next() {
			var id, group, site string
			var slug sql.NullString
			var raw []byte
			rows.Scan(&id, &group, &site, &slug, &raw)
			var d struct{ Title string }
			json.Unmarshal(raw, &d)
			label := d.Title
			if label == "" {
				label = slug.String
			}
			if label == "" {
				label = id
			}
			out = append(out, map[string]string{"id": id, "group": group, "site": site, "label": label})
		}
		return out
	}

	a.Admin(mux, "GET "+prefix+"/api/content/{handle}", func(w http.ResponseWriter, r *http.Request) {
		handle, title, b, err := resolve(r, false)
		if err != nil {
			reply(w, nil, err)
			return
		}
		ss := src.Sites(handle)
		active, origin, isOrigin := resolveSite(ss, r.URL.Query().Get("site"))
		var bardContainer string // container gambar editor Bard: yang pertama menurut handle
		a.DB.QueryRow(`SELECT handle FROM asset_containers ORDER BY handle LIMIT 1`).Scan(&bardContainer)
		authors := []map[string]string{}
		if rows, e := a.DB.Query(`SELECT id,name FROM users WHERE banned_at IS NULL ORDER BY name`); e == nil {
			for rows.Next() {
				var id, name string
				rows.Scan(&id, &name)
				authors = append(authors, map[string]string{"id": id, "name": name})
			}
			rows.Close()
		}
		out := map[string]any{
			"item":      map[string]any{"handle": handle, "title": title},
			"blueprint": b, "sites": ss, "activeSite": active, "isOrigin": isOrigin,
			"data":    loadContent(a, src, handle, b.Fields, active, origin),
			"canEdit": src.CanEdit(web.CurrentUser(r), handle),
			"entries": labelled(`SELECT id,collection,site,slug,data FROM entries ORDER BY collection, slug`),
			"terms":   labelled(`SELECT id,taxonomy,site,slug,data FROM taxonomy_terms ORDER BY slug`),
			"authors": authors, "bardContainer": bardContainer,
		}
		if src.Extra != nil {
			for k, v := range src.Extra(handle) {
				out[k] = v
			}
		}
		reply(w, out, nil)
	})

	a.Admin(mux, "POST "+prefix+"/api/content/{handle}", func(w http.ResponseWriter, r *http.Request) {
		handle, _, b, err := resolve(r, true)
		if err != nil {
			reply(w, nil, err)
			return
		}
		var in struct{ Data map[string]any }
		if err := decode(w, r, &in); err != nil {
			reply(w, nil, err)
			return
		}
		active, origin, isOrigin := resolveSite(src.Sites(handle), r.URL.Query().Get("site"))
		c := &chk{a: a, site: active, errs: map[string]string{}}
		data := c.check(b.Fields, in.Data, "")
		if len(c.errs) > 0 {
			reply(w, nil, fieldErrs{msg: c.first, all: c.errs})
			return
		}
		if !isOrigin { // simpan hanya field yang benar-benar dioverride; sisanya tetap mengikuti bahasa default
			originData := src.Load(handle, origin)
			localizeLinks(a, originData, b.Fields, active)
			skip, over := nonLocalizable(b.Fields), map[string]any{}
			for k, v := range data {
				if ov, ok := originData[k]; !skip[k] && !(ok && eq(ov, v)) {
					over[k] = v
				}
			}
			data = over
		}
		if err = src.Save(handle, active, data); err == nil && src.FlashOnSave != nil {
			if m := src.FlashOnSave(handle); m != "" {
				web.Flash(w, m)
			}
		}
		reply(w, nil, err)
	})
}
