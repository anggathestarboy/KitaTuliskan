// Package sitesettings = app/Modules/SiteSettings: SiteSettingsManager (robots.txt, llms.txt, schema.org, integrations),
// LlmsTxtGenerator, SchemaMarkupBuilder; publik /robots.txt, /llms.txt, dan /api/site-settings (JSON-LD + integrations untuk Astro).
package sitesettings

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"

	"draftcms-lite/internal/modules/activitylog"
	"draftcms-lite/internal/web"
)

var SchemaTypes = []string{"Organization", "WebSite", "ContactPage", "AboutPage", "Service", "CollectionPage"}

// Block = satu schema.org block (HasSchemaBlocks::defaultSchemaBlock): semua key selalu ada.
type Block struct {
	Type            string   `json:"type"`
	Name            string   `json:"name"`
	AlternateName   string   `json:"alternate_name"`
	Description     string   `json:"description"`
	Logo            string   `json:"logo"`
	SameAs          []string `json:"same_as"`
	Email           string   `json:"email"`
	Telephone       string   `json:"telephone"`
	ContactType     string   `json:"contact_type"`
	AddressStreet   string   `json:"address_street"`
	AddressLocality string   `json:"address_locality"`
	AddressRegion   string   `json:"address_region"`
	AddressPostal   string   `json:"address_postal"`
	AddressCountry  string   `json:"address_country"`
	ServiceType     string   `json:"service_type"`
	AreaServed      string   `json:"area_served"`
}

// State = properti publik SiteSettingsManager; dikirim utuh sebagai JSON oleh Alpine saat save.
type State struct {
	RobotsTxt             string            `json:"robotsTxt"`
	LlmsTxt               string            `json:"llmsTxt"`
	LlmsCollections       []string          `json:"llmsCollections"`
	LlmsTaxonomies        []string          `json:"llmsTaxonomies"`
	LlmsLimit             Num               `json:"llmsLimit"`
	LlmsAdditionalContent string            `json:"llmsAdditionalContent"`
	SchemaBlocks          []Block           `json:"schemaBlocks"`
	GaEnabled             bool              `json:"gaEnabled"`
	GaMeasurementID       string            `json:"gaMeasurementId"`
	GscEnabled            bool              `json:"gscEnabled"`
	GscVerificationCode   string            `json:"gscVerificationCode"`
	Errors                map[string]string `json:"errors"`
}

// Num: angka dari Alpine bisa datang sebagai string ("1000", "" saat dikosongkan) atau number; json.Number menolak "".
type Num string

func (n *Num) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch t := v.(type) {
	case string:
		*n = Num(strings.TrimSpace(t))
	case float64:
		*n = Num(strconv.FormatFloat(t, 'f', -1, 64))
	default:
		*n = ""
	}
	return nil
}

func (n Num) Int64() (int64, error) { return strconv.ParseInt(string(n), 10, 64) }

func appName() string { return or(os.Getenv("APP_NAME"), "Draft CMS") }
func appURL() string  { return strings.TrimRight(os.Getenv("APP_URL"), "/") }
func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func Register(mux *http.ServeMux, a *web.App) {
	a.Admin(mux, "GET /site-settings", func(w http.ResponseWriter, r *http.Request) {
		render(a, w, r, load(a))
	})
	a.Admin(mux, "POST /site-settings", func(w http.ResponseWriter, r *http.Request) {
		var st State
		if err := json.Unmarshal([]byte(r.FormValue("state")), &st); err != nil {
			http.Error(w, "Bad state: "+err.Error(), 400)
			return
		}
		st.Errors = validate(st)
		if len(st.Errors) > 0 {
			render(a, w, r, st)
			return
		}
		limit, _ := st.LlmsLimit.Int64()
		llmsConfig, _ := json.Marshal(map[string]any{"collections": nonNil(st.LlmsCollections), "taxonomies": nonNil(st.LlmsTaxonomies), "limit": limit, "additional_content": nullable(st.LlmsAdditionalContent)})
		for i := range st.SchemaBlocks { // cleanedSchemaBlocks(): buang sameAs kosong
			st.SchemaBlocks[i].SameAs = compact(st.SchemaBlocks[i].SameAs)
		}
		schema, _ := json.Marshal(nonNilBlocks(st.SchemaBlocks))
		integrations, _ := json.Marshal(map[string]any{"ga_enabled": st.GaEnabled, "ga_measurement_id": nullable(st.GaMeasurementID), "gsc_enabled": st.GscEnabled, "gsc_verification_code": nullable(st.GscVerificationCode)})
		ensureRow(a)
		a.DB.Exec(`UPDATE site_settings SET robots_txt=?,llms_txt=?,llms_config=?,`+"`schema`"+`=?,integrations=?,updated_at=? ORDER BY id LIMIT 1`,
			nullable(st.RobotsTxt), nullable(st.LlmsTxt), string(llmsConfig), string(schema), string(integrations), web.Now())
		activitylog.Record(a, web.CurrentUser(r).ID, "updated", "site_setting", "", "site settings", nil)
		web.Flash(w, "Site settings saved.")
		http.Redirect(w, r, a.CP("/site-settings?q="+or(r.FormValue("q"), "robots")), http.StatusFound)
	})
	// generateLlms(): dipanggil via fetch, hasilnya mengisi textarea llms.txt di client.
	a.Admin(mux, "POST /site-settings/generate", func(w http.ResponseWriter, r *http.Request) {
		var st State
		json.Unmarshal([]byte(r.FormValue("state")), &st)
		limit, _ := st.LlmsLimit.Int64()
		if limit < 1 {
			limit = 1000
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(Generate(a, st.LlmsCollections, st.LlmsTaxonomies, int(limit), st.LlmsAdditionalContent)))
	})

	txt := func(col string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var s string
			a.DB.QueryRow("SELECT COALESCE(" + col + ",'') FROM site_settings ORDER BY id LIMIT 1").Scan(&s)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte(s))
		}
	}
	mux.HandleFunc("GET /robots.txt", txt("robots_txt"))
	mux.HandleFunc("GET /llms.txt", txt("llms_txt"))
	// = structuredDataBlocks() + integrations untuk <head> halaman publik (di source lewat Blade; di sini Astro fetch saat build).
	mux.HandleFunc("GET /api/site-settings", func(w http.ResponseWriter, r *http.Request) {
		st := load(a)
		var integ map[string]any
		rows, _ := web.Rows(a.DB, `SELECT integrations FROM site_settings ORDER BY id LIMIT 1`)
		if len(rows) > 0 {
			json.Unmarshal([]byte(str(rows[0]["integrations"])), &integ)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"schema": BuildAll(st.SchemaBlocks), "integrations": integ})
	})
}

func render(a *web.App, w http.ResponseWriter, r *http.Request, st State) {
	titled := func(table string) []map[string]string {
		rows, _ := web.Rows(a.DB, "SELECT handle,title FROM "+table+" ORDER BY title")
		out := []map[string]string{}
		for _, r := range rows {
			out = append(out, map[string]string{"handle": str(r["handle"]), "title": str(r["title"])})
		}
		return out
	}
	var container string
	a.DB.QueryRow(`SELECT handle FROM asset_containers ORDER BY handle LIMIT 1`).Scan(&container)
	a.Render(w, r, "sitesettings/index.html", map[string]any{"State": st, "Tab": or(r.FormValue("q"), "robots"), "Collections": titled("collections"), "Taxonomies": titled("taxonomies"),
		"Opts": map[string]any{"appName": appName(), "appUrl": appURL(), "assetContainer": container, "schemaTypes": SchemaTypes}})
}

func str(v any) string { s, _ := v.(string); return s }

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilBlocks(b []Block) []Block {
	if b == nil {
		return []Block{}
	}
	return b
}

func compact(in []string) []string {
	out := []string{}
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ensureRow = SiteSetting::current(): firstOrCreate([]).
func ensureRow(a *web.App) {
	var n int
	a.DB.QueryRow(`SELECT COUNT(*) FROM site_settings`).Scan(&n)
	if n == 0 {
		a.DB.Exec(`INSERT INTO site_settings(created_at,updated_at) VALUES(?,?)`, web.Now(), web.Now())
	}
}

// load = mount(): baris tunggal → State. Schema format lama (satu objek, bukan list) diangkat jadi list satu item.
func load(a *web.App) State {
	st := State{LlmsCollections: []string{}, LlmsTaxonomies: []string{}, LlmsLimit: "1000", SchemaBlocks: []Block{}, Errors: map[string]string{}}
	rows, _ := web.Rows(a.DB, `SELECT * FROM site_settings ORDER BY id LIMIT 1`)
	if len(rows) == 0 {
		return st
	}
	row := rows[0]
	st.RobotsTxt, st.LlmsTxt = str(row["robots_txt"]), str(row["llms_txt"])
	var cfg struct {
		Collections       []string `json:"collections"`
		Taxonomies        []string `json:"taxonomies"`
		Limit             Num      `json:"limit"`
		AdditionalContent string   `json:"additional_content"`
	}
	if json.Unmarshal([]byte(str(row["llms_config"])), &cfg) == nil {
		st.LlmsCollections, st.LlmsTaxonomies, st.LlmsAdditionalContent = nonNil(cfg.Collections), nonNil(cfg.Taxonomies), cfg.AdditionalContent
		if cfg.Limit != "" {
			st.LlmsLimit = cfg.Limit
		}
	}
	raw := str(row["schema"])
	if strings.HasPrefix(strings.TrimSpace(raw), "[") {
		json.Unmarshal([]byte(raw), &st.SchemaBlocks)
	} else if raw != "" {
		var one Block
		if json.Unmarshal([]byte(raw), &one) == nil && (one.Type != "" || one.Name != "") {
			st.SchemaBlocks = []Block{one}
		}
	}
	for i := range st.SchemaBlocks {
		if st.SchemaBlocks[i].Type == "" {
			st.SchemaBlocks[i].Type = "Organization"
		}
		st.SchemaBlocks[i].SameAs = nonNil(st.SchemaBlocks[i].SameAs)
	}
	var integ struct {
		GaEnabled           bool   `json:"ga_enabled"`
		GaMeasurementID     string `json:"ga_measurement_id"`
		GscEnabled          bool   `json:"gsc_enabled"`
		GscVerificationCode string `json:"gsc_verification_code"`
	}
	json.Unmarshal([]byte(str(row["integrations"])), &integ)
	st.GaEnabled, st.GaMeasurementID, st.GscEnabled, st.GscVerificationCode = integ.GaEnabled, integ.GaMeasurementID, integ.GscEnabled, integ.GscVerificationCode
	return st
}

// validate = save() rules + schemaBlockValidationRules().
func validate(st State) map[string]string {
	errs := map[string]string{}
	if n, err := st.LlmsLimit.Int64(); st.LlmsLimit == "" || err != nil || n < 1 {
		errs["llmsLimit"] = "The entries/terms limit must be at least 1."
	}
	if st.GaEnabled && strings.TrimSpace(st.GaMeasurementID) == "" {
		errs["gaMeasurementId"] = "The measurement ID is required when Google Analytics is enabled."
	}
	if st.GscEnabled && strings.TrimSpace(st.GscVerificationCode) == "" {
		errs["gscVerificationCode"] = "The verification code is required when Google Search Console is enabled."
	}
	if len(st.GaMeasurementID) > 255 {
		errs["gaMeasurementId"] = "The measurement ID must not be greater than 255 characters."
	}
	if len(st.GscVerificationCode) > 255 {
		errs["gscVerificationCode"] = "The verification code must not be greater than 255 characters."
	}
	for i, b := range st.SchemaBlocks {
		k := func(f string) string { return "schemaBlocks." + web.Itoa(int64(i)) + "." + f }
		ok := false
		for _, t := range SchemaTypes {
			ok = ok || t == b.Type
		}
		if !ok {
			errs[k("type")] = "The selected schema type is invalid."
		}
		if b.Email != "" {
			if _, err := mail.ParseAddress(b.Email); err != nil || len(b.Email) > 255 {
				errs[k("email")] = "The email must be a valid email address."
			}
		}
		for f, max := range map[string]int{"name": 255, "alternate_name": 255, "description": 1000, "logo": 2048, "telephone": 50, "contact_type": 255, "address_street": 255, "address_locality": 255, "address_region": 255, "address_postal": 50, "address_country": 255, "service_type": 255, "area_served": 255} {
			if len(fieldOf(b, f)) > max {
				errs[k(f)] = "The " + strings.ReplaceAll(f, "_", " ") + " must not be greater than " + web.Itoa(int64(max)) + " characters."
			}
		}
		for _, u := range b.SameAs {
			if len(u) > 2048 {
				errs[k("same_as")] = "Each profile URL must not be greater than 2048 characters."
			}
		}
	}
	return errs
}

func fieldOf(b Block, f string) string {
	m := map[string]string{"name": b.Name, "alternate_name": b.AlternateName, "description": b.Description, "logo": b.Logo, "telephone": b.Telephone, "contact_type": b.ContactType, "address_street": b.AddressStreet, "address_locality": b.AddressLocality, "address_region": b.AddressRegion, "address_postal": b.AddressPostal, "address_country": b.AddressCountry, "service_type": b.ServiceType, "area_served": b.AreaServed}
	return m[f]
}

// Generate = LlmsTxtGenerator::generate(). URL entry = site.url + entry.uri (ponytail: EntryUrlResolver penuh belum diport; entry tanpa uri ditulis tanpa link).
func Generate(a *web.App, collections, taxonomies []string, limit int, additional string) string {
	sections := []string{"# " + appName()}
	siteURL := map[string]string{}
	sites, _ := web.Rows(a.DB, `SELECT handle,url FROM sites`)
	for _, s := range sites {
		siteURL[str(s["handle"])] = strings.TrimRight(str(s["url"]), "/")
	}
	for _, h := range collections {
		var title string
		if a.DB.QueryRow(`SELECT title FROM collections WHERE handle=?`, h).Scan(&title) != nil {
			continue
		}
		rows, _ := web.Rows(a.DB, `SELECT id,slug,uri,site,data FROM entries WHERE collection=? AND status='published' LIMIT ?`, h, limit)
		if len(rows) == 0 {
			continue
		}
		lines := []string{"## " + title}
		for _, e := range rows {
			t := or(titleFrom(e["data"]), or(str(e["slug"]), str(e["id"])))
			if uri := str(e["uri"]); uri != "" {
				lines = append(lines, "- ["+t+"]("+siteURL[str(e["site"])]+uri+")")
			} else {
				lines = append(lines, "- "+t)
			}
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	for _, h := range taxonomies {
		var title string
		if a.DB.QueryRow(`SELECT title FROM taxonomies WHERE handle=?`, h).Scan(&title) != nil {
			continue
		}
		rows, _ := web.Rows(a.DB, `SELECT id,slug,uri,data FROM taxonomy_terms WHERE taxonomy=? LIMIT ?`, h, limit)
		if len(rows) == 0 {
			continue
		}
		lines := []string{"## " + title}
		for _, t := range rows {
			title := or(titleFrom(t["data"]), or(str(t["slug"]), str(t["id"])))
			if uri := str(t["uri"]); uri != "" {
				lines = append(lines, "- ["+title+"]("+uri+")")
			} else {
				lines = append(lines, "- "+title)
			}
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	if s := strings.TrimSpace(additional); s != "" {
		sections = append(sections, s)
	}
	return strings.Join(sections, "\n\n") + "\n"
}

func titleFrom(data any) string {
	var d struct {
		Title string `json:"title"`
	}
	json.Unmarshal([]byte(str(data)), &d)
	return d.Title
}

// BuildAll/Build = SchemaMarkupBuilder: block tanpa name dilewati.
func BuildAll(blocks []Block) []map[string]any {
	out := []map[string]any{}
	for _, b := range blocks {
		if m := Build(b); m != nil {
			out = append(out, m)
		}
	}
	return out
}

func Build(b Block) map[string]any {
	name := strings.TrimSpace(b.Name)
	if name == "" {
		return nil
	}
	lang := or(os.Getenv("APP_LOCALE"), "en")
	set := func(m map[string]any, k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	m := map[string]any{"@context": "https://schema.org", "name": name, "url": appURL()}
	sameAs := compact(b.SameAs)
	switch b.Type {
	case "WebSite":
		m["@type"], m["inLanguage"] = "WebSite", lang
		set(m, "description", b.Description)
	case "ContactPage", "AboutPage", "CollectionPage":
		m["@type"], m["inLanguage"] = b.Type, lang
		set(m, "description", b.Description)
		set(m, "primaryImageOfPage", b.Logo)
	case "Service":
		m["@type"] = "Service"
		set(m, "description", b.Description)
		set(m, "serviceType", b.ServiceType)
		if b.AreaServed != "" {
			m["areaServed"] = map[string]any{"@type": "AdministrativeArea", "name": b.AreaServed}
		}
		p := map[string]any{"@type": "Organization", "name": name, "url": appURL()}
		set(p, "description", b.Description)
		if len(sameAs) > 0 {
			p["sameAs"] = sameAs
		}
		m["provider"] = p
	default:
		m["@type"] = "Organization"
		set(m, "alternateName", b.AlternateName)
		set(m, "description", b.Description)
		set(m, "logo", b.Logo)
		set(m, "email", b.Email)
		if len(sameAs) > 0 {
			m["sameAs"] = sameAs
		}
		cp := map[string]any{"@type": "ContactPoint"}
		set(cp, "telephone", b.Telephone)
		set(cp, "contactType", b.ContactType)
		set(cp, "email", b.Email)
		if len(cp) > 1 {
			m["contactPoint"] = []map[string]any{cp}
		}
		addr := map[string]any{"@type": "PostalAddress"}
		set(addr, "streetAddress", b.AddressStreet)
		set(addr, "addressLocality", b.AddressLocality)
		set(addr, "addressRegion", b.AddressRegion)
		set(addr, "postalCode", b.AddressPostal)
		set(addr, "addressCountry", b.AddressCountry)
		if len(addr) > 1 {
			m["address"] = addr
		}
	}
	return m
}
