// DIGENERATE oleh plugins/cms-types.mjs dari blueprint CMS. Jangan diedit.
interface CmsAsset { url: string; alt?: string; width?: number; height?: number; loading?: string; title?: string; mime_type?: string; [k: string]: any }
type CmsEntry<T> = import('./cms').Entry & T & { url: string; data: T };
declare namespace Cms {
  type Navigation = { "sidebar-admin": any; "header": { "site_name"?: string; "site_image"?: CmsAsset[]; "menu_loop"?: Array<{ _enabled?: boolean; "menu_label"?: string; "menu_url"?: string; }>; "bard_multi_language"?: string; "bard_indo_only"?: string; "tambahkan_field"?: boolean; "field_tambahan"?: string; }; };
  type Global = { "global_sidebar": { "site_image"?: CmsAsset | null; "menu_loop"?: Array<{ _enabled?: boolean; "menu_label"?: string; "menu_child_menu"?: Array<{ _enabled?: boolean; "menu_child_label"?: string; }>; }>; }; "test_global": { "waduh_bisa_gak_ya"?: string; }; };
  type Collection = { "pages": CmsEntry<{ "about_page"?: string; "hero_image"?: CmsAsset | null; "hero_text"?: string; }>[]; "news": CmsEntry<{ "news_content"?: string; "looping_tags_coba"?: Array<{ _enabled?: boolean; "waw"?: string; }>; }>[]; };
  type Taxonomy = { "breaking_news": Array<Record<string, any> & { "news_label"?: string; "news_desc"?: string; }>; "csr": Array<Record<string, any> & {  }>; };
}
