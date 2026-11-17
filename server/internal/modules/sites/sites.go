// Package sites = app/Modules/Sites: SiteManager (create/edit/delete/reorder). Origin = site dengan order terkecil, tidak bisa dihapus.
package sites

import (
	"encoding/json"
	"net/http"
	"strings"

	"draftcms-lite/internal/web"
)

// Languages = Symfony Intl Languages::getNames() di source, dipangkas ke yang umum. "code|Name" untuk partial select.
var Languages = []string{"ar|Arabic", "bn|Bengali", "zh|Chinese", "cs|Czech", "da|Danish", "nl|Dutch", "en|English", "fi|Finnish", "fr|French", "de|German", "el|Greek", "he|Hebrew", "hi|Hindi", "hu|Hungarian", "id|Indonesian", "it|Italian", "ja|Japanese", "jv|Javanese", "ko|Korean", "ms|Malay", "no|Norwegian", "fa|Persian", "pl|Polish", "pt|Portuguese", "ro|Romanian", "ru|Russian", "es|Spanish", "sv|Swedish", "tl|Tagalog", "th|Thai", "tr|Turkish", "uk|Ukrainian", "ur|Urdu", "vi|Vietnamese"}

func Register(mux *http.ServeMux, a *web.App) {
	base := a.CP("/sites")
	guard := func(w http.ResponseWriter, r *http.Request) bool {
		if !web.CurrentUser(r).Can("configure sites") {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return false
		}
		return true
	}
	originID := func() (id int64) {
		a.DB.QueryRow("SELECT id FROM sites ORDER BY `order`, id LIMIT 1").Scan(&id)
		return
	}
	a.Admin(mux, "GET /sites", func(w http.ResponseWriter, r *http.Request) {
		rows, _ := web.Rows(a.DB, "SELECT * FROM sites ORDER BY `order`, id")
		for _, row := range rows { // attributes JSON → [{key,value}] untuk form
			var m map[string]any
			json.Unmarshal([]byte(row["attributes"].(string)), &m)
			attrs := []map[string]any{}
			for k, v := range m {
				attrs = append(attrs, map[string]any{"key": k, "value": v})
			}
			row["attrs"] = attrs
		}
		a.Render(w, r, "sites/index.html", map[string]any{"Rows": rows, "Base": base, "Origin": originID(), "Languages": Languages, "CanConfigure": web.CurrentUser(r).Can("configure sites")})
	})
	// POST = SiteManager::save(): id kosong → insert (order = max+1), ada → update. Handle origin tidak boleh berubah.
	a.Admin(mux, "POST /sites", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		defer http.Redirect(w, r, base, http.StatusFound)
		r.ParseForm()
		f := func(k string) string { return strings.TrimSpace(r.FormValue(k)) }
		id, handle := f("id"), f("handle")
		if f("name") == "" || f("url") == "" || f("locale") == "" || f("lang") == "" || !web.ValidHandle(handle) {
			web.Flash(w, "Name, URL, locale, language and a valid handle (lowercase, numbers, - or _) are required.")
			return
		}
		attrs := map[string]string{}
		for i, k := range r.Form["attr_key"] {
			if k = strings.TrimSpace(k); k != "" && i < len(r.Form["attr_value"]) {
				attrs[k] = r.Form["attr_value"][i]
			}
		}
		attrJSON, _ := json.Marshal(attrs)
		var err error
		if id != "" {
			if id == web.Itoa(originID()) {
				a.DB.QueryRow("SELECT handle FROM sites WHERE id=?", id).Scan(&handle)
			}
			_, err = a.DB.Exec("UPDATE sites SET name=?,handle=?,url=?,locale=?,lang=?,attributes=?,updated_at=? WHERE id=?", f("name"), handle, f("url"), f("locale"), f("lang"), attrJSON, web.Now(), id)
		} else {
			_, err = a.DB.Exec("INSERT INTO sites(name,handle,url,locale,lang,attributes,`order`,created_at,updated_at) SELECT ?,?,?,?,?,?,COALESCE(MAX(`order`),0)+1,?,? FROM sites", f("name"), handle, f("url"), f("locale"), f("lang"), attrJSON, web.Now(), web.Now())
		}
		if err != nil {
			web.Flash(w, err.Error())
		} else {
			web.Flash(w, "Site saved.")
		}
	})
	a.Admin(mux, "POST /sites/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		if r.PathValue("id") == web.Itoa(originID()) {
			web.Flash(w, "The origin site cannot be deleted.")
		} else if _, err := a.DB.Exec("DELETE FROM sites WHERE id=?", r.PathValue("id")); err != nil {
			web.Flash(w, err.Error())
		} else {
			web.Flash(w, "Site deleted.")
		}
		http.Redirect(w, r, base, http.StatusFound)
	})
	// move: tukar `order` dengan tetangga di atas/bawah = swapOrder().
	a.Admin(mux, "POST /sites/{id}/move/{dir}", func(w http.ResponseWriter, r *http.Request) {
		if !guard(w, r) {
			return
		}
		defer http.Redirect(w, r, base, http.StatusFound)
		var id, ord, nid, nord int64
		if a.DB.QueryRow("SELECT id,`order` FROM sites WHERE id=?", r.PathValue("id")).Scan(&id, &ord) != nil {
			return
		}
		q := "SELECT id,`order` FROM sites WHERE `order`>? ORDER BY `order` ASC LIMIT 1"
		if r.PathValue("dir") == "up" {
			q = "SELECT id,`order` FROM sites WHERE `order`<? ORDER BY `order` DESC LIMIT 1"
		}
		if a.DB.QueryRow(q, ord).Scan(&nid, &nord) != nil {
			return
		}
		a.DB.Exec("UPDATE sites SET `order`=? WHERE id=?", nord, id)
		a.DB.Exec("UPDATE sites SET `order`=? WHERE id=?", ord, nid)
	})
}
