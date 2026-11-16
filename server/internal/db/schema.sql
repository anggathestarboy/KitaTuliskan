-- Disalin 1:1 dari source/database/migrations (MySQL dialect)
CREATE TABLE IF NOT EXISTS users (
  id CHAR(36) PRIMARY KEY, name VARCHAR(255) NOT NULL, email VARCHAR(255) NOT NULL UNIQUE, email_verified_at TIMESTAMP NULL, banned_at TIMESTAMP NULL,
  password VARCHAR(255) NOT NULL, two_factor_secret TEXT, two_factor_recovery_codes TEXT, two_factor_confirmed_at TIMESTAMP NULL,
  preferences JSON, remember_token VARCHAR(100), created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL
);
CREATE TABLE IF NOT EXISTS password_reset_tokens (email VARCHAR(255) PRIMARY KEY, token VARCHAR(255) NOT NULL, created_at TIMESTAMP NULL);
CREATE TABLE IF NOT EXISTS sessions (
  id VARCHAR(255) PRIMARY KEY, user_id CHAR(36), ip_address VARCHAR(45), user_agent TEXT, payload LONGTEXT NOT NULL, last_activity INT NOT NULL,
  INDEX sessions_user_id_index (user_id), INDEX sessions_last_activity_index (last_activity)
);
CREATE TABLE IF NOT EXISTS passkeys (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, user_id CHAR(36) NOT NULL, name VARCHAR(255) NOT NULL, credential_id VARCHAR(255) NOT NULL UNIQUE,
  credential JSON NOT NULL, last_used_at TIMESTAMP NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  INDEX passkeys_user_id_index (user_id), CONSTRAINT passkeys_user_id_foreign FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS cache (`key` VARCHAR(255) PRIMARY KEY, value MEDIUMTEXT NOT NULL, expiration BIGINT NOT NULL, INDEX cache_expiration_index (expiration));
CREATE TABLE IF NOT EXISTS cache_locks (`key` VARCHAR(255) PRIMARY KEY, owner VARCHAR(255) NOT NULL, expiration BIGINT NOT NULL, INDEX cache_locks_expiration_index (expiration));
CREATE TABLE IF NOT EXISTS jobs (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, queue VARCHAR(255) NOT NULL, payload LONGTEXT NOT NULL, attempts SMALLINT UNSIGNED NOT NULL, reserved_at INT UNSIGNED, available_at INT UNSIGNED NOT NULL, created_at INT UNSIGNED NOT NULL, INDEX jobs_queue_index (queue));
CREATE TABLE IF NOT EXISTS job_batches (id VARCHAR(255) PRIMARY KEY, name VARCHAR(255) NOT NULL, total_jobs INT NOT NULL, pending_jobs INT NOT NULL, failed_jobs INT NOT NULL, failed_job_ids LONGTEXT NOT NULL, options MEDIUMTEXT, cancelled_at INT, created_at INT NOT NULL, finished_at INT);
CREATE TABLE IF NOT EXISTS failed_jobs (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, uuid VARCHAR(255) NOT NULL UNIQUE, `connection` TEXT NOT NULL, queue TEXT NOT NULL, payload LONGTEXT NOT NULL, exception LONGTEXT NOT NULL, failed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);

-- Users module: roles, groups, pivots (migrations 05-09)
CREATE TABLE IF NOT EXISTS roles (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, permissions JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY roles_handle_unique (handle));
CREATE TABLE IF NOT EXISTS user_groups (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY user_groups_handle_unique (handle));
CREATE TABLE IF NOT EXISTS role_user (role_id BIGINT UNSIGNED NOT NULL, user_id CHAR(36) NOT NULL, PRIMARY KEY (role_id, user_id),
  CONSTRAINT role_user_role_id_foreign FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE, CONSTRAINT role_user_user_id_foreign FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS role_user_group (role_id BIGINT UNSIGNED NOT NULL, user_group_id BIGINT UNSIGNED NOT NULL, PRIMARY KEY (role_id, user_group_id),
  CONSTRAINT role_user_group_role_id_foreign FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE, CONSTRAINT role_user_group_user_group_id_foreign FOREIGN KEY (user_group_id) REFERENCES user_groups(id) ON DELETE CASCADE);
CREATE TABLE IF NOT EXISTS user_group_user (user_group_id BIGINT UNSIGNED NOT NULL, user_id CHAR(36) NOT NULL, PRIMARY KEY (user_group_id, user_id),
  CONSTRAINT user_group_user_user_group_id_foreign FOREIGN KEY (user_group_id) REFERENCES user_groups(id) ON DELETE CASCADE, CONSTRAINT user_group_user_user_id_foreign FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE);

CREATE TABLE IF NOT EXISTS collections (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, settings JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY collections_handle_unique (handle));
CREATE TABLE IF NOT EXISTS entries (
  id CHAR(36) PRIMARY KEY COMMENT 'UUID', site VARCHAR(255) NOT NULL, origin_id CHAR(36) NULL COMMENT 'Self-ref for localization (nullable)', published TINYINT NOT NULL DEFAULT 1, status VARCHAR(255) NOT NULL DEFAULT 'draft',
  slug VARCHAR(255), uri VARCHAR(255), `date` VARCHAR(255), `order` INT, collection VARCHAR(255) NOT NULL, blueprint VARCHAR(30), data JSON NOT NULL, `schema` JSON, seo JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  INDEX entries_site_index (site), INDEX entries_origin_id_index (origin_id), INDEX entries_uri_index (uri), INDEX entries_date_index (`date`), INDEX entries_order_index (`order`),
  INDEX entries_collection_index (collection), INDEX entries_blueprint_index (blueprint),
  CONSTRAINT entries_collection_foreign FOREIGN KEY (collection) REFERENCES collections(handle)
);
CREATE TABLE IF NOT EXISTS taxonomies (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, sites JSON, settings JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY taxonomies_handle_unique (handle));
CREATE TABLE IF NOT EXISTS taxonomy_terms (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, site VARCHAR(255) NOT NULL, origin_id BIGINT UNSIGNED NULL COMMENT 'Self-ref for localization (nullable)', slug VARCHAR(255) NOT NULL, uri VARCHAR(255), taxonomy VARCHAR(255) NOT NULL, blueprint VARCHAR(30), data JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  UNIQUE KEY taxonomy_terms_slug_taxonomy_site_unique (slug, taxonomy, site), INDEX taxonomy_terms_site_index (site), INDEX taxonomy_terms_origin_id_index (origin_id), INDEX taxonomy_terms_uri_index (uri), INDEX taxonomy_terms_taxonomy_index (taxonomy),
  CONSTRAINT taxonomy_terms_taxonomy_foreign FOREIGN KEY (taxonomy) REFERENCES taxonomies(handle),
  CONSTRAINT taxonomy_terms_origin_id_foreign FOREIGN KEY (origin_id) REFERENCES taxonomy_terms(id)
);
CREATE TABLE IF NOT EXISTS global_sets (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, settings JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY global_sets_handle_unique (handle));
CREATE TABLE IF NOT EXISTS global_set_variables (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, locale VARCHAR(255), data JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  INDEX global_set_variables_handle_index (handle), CONSTRAINT global_set_variables_handle_foreign FOREIGN KEY (handle) REFERENCES global_sets(handle)
);
CREATE TABLE IF NOT EXISTS asset_containers (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, disk VARCHAR(255) NOT NULL, settings JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY asset_containers_handle_unique (handle));
CREATE TABLE IF NOT EXISTS assets_meta (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, container VARCHAR(255) NOT NULL, folder VARCHAR(255) NOT NULL, basename VARCHAR(255) NOT NULL, filename VARCHAR(255) NOT NULL,
  extension CHAR(10) NOT NULL, path VARCHAR(255) NOT NULL, meta JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  UNIQUE KEY assets_meta_container_folder_basename_unique (container, folder, basename),
  INDEX assets_meta_container_index (container), INDEX assets_meta_folder_index (folder), INDEX assets_meta_basename_index (basename),
  INDEX assets_meta_filename_index (filename), INDEX assets_meta_extension_index (extension), INDEX assets_meta_path_index (path),
  CONSTRAINT assets_meta_container_foreign FOREIGN KEY (container) REFERENCES asset_containers(handle)
);
CREATE TABLE IF NOT EXISTS blueprints (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, namespace VARCHAR(255), handle VARCHAR(255) NOT NULL, data JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY blueprints_handle_namespace_unique (handle, namespace), INDEX blueprints_namespace_index (namespace));
CREATE TABLE IF NOT EXISTS fieldsets (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, data JSON NOT NULL, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY fieldsets_handle_unique (handle));
CREATE TABLE IF NOT EXISTS forms (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, settings JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY forms_handle_unique (handle));
CREATE TABLE IF NOT EXISTS form_submissions (
  id DECIMAL(14,4) PRIMARY KEY, form VARCHAR(255), data JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  UNIQUE KEY form_submissions_form_created_at_unique (form, created_at), INDEX form_submissions_form_index (form),
  CONSTRAINT form_submissions_form_foreign FOREIGN KEY (form) REFERENCES forms(handle)
);
CREATE TABLE IF NOT EXISTS navigations (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, title VARCHAR(255) NOT NULL, settings JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY navigations_handle_unique (handle));
CREATE TABLE IF NOT EXISTS trees (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, type VARCHAR(255) NOT NULL COMMENT 'collection or navigation', locale VARCHAR(255), tree JSON, settings JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  UNIQUE KEY trees_handle_type_locale_unique (handle, type, locale), INDEX trees_type_index (type), INDEX trees_locale_index (locale)
);
CREATE TABLE IF NOT EXISTS sites (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, handle VARCHAR(255) NOT NULL, name VARCHAR(255) NOT NULL, url VARCHAR(255) NOT NULL, locale VARCHAR(255) NOT NULL, lang VARCHAR(255) NOT NULL,
  attributes JSON NOT NULL, `order` INT NOT NULL DEFAULT 0, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY sites_handle_unique (handle), INDEX sites_order_index (`order`)
);
CREATE TABLE IF NOT EXISTS sitemaps (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, origin_id CHAR(36) NULL COMMENT 'References entries.id (nullable)', collection VARCHAR(255) NOT NULL, url VARCHAR(255) NOT NULL,
  changefreq VARCHAR(255) NOT NULL DEFAULT 'weekly', priority DECIMAL(2,1) NOT NULL DEFAULT 0.5, lastmod TIMESTAMP NULL, is_active TINYINT NOT NULL DEFAULT 1, site VARCHAR(255) NOT NULL DEFAULT 'indonesia',
  created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  UNIQUE KEY sitemaps_site_collection_origin_id_unique (site, collection, origin_id), INDEX sitemaps_site_collection_url_index (site, collection, url),
  CONSTRAINT sitemaps_collection_foreign FOREIGN KEY (collection) REFERENCES collections(handle), CONSTRAINT sitemaps_site_foreign FOREIGN KEY (site) REFERENCES sites(handle)
);
CREATE TABLE IF NOT EXISTS redirects (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, source VARCHAR(255) NOT NULL, target VARCHAR(255) NOT NULL, status_code INT NOT NULL DEFAULT 301, match_type VARCHAR(255) NOT NULL DEFAULT 'exact' COMMENT 'exact | prefix | regex',
  is_active TINYINT NOT NULL DEFAULT 1, hits BIGINT NOT NULL DEFAULT 0, start_at TIMESTAMP NULL, end_at TIMESTAMP NULL, is_external TINYINT NOT NULL DEFAULT 0, keep_path TINYINT NOT NULL DEFAULT 1,
  created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL, UNIQUE KEY redirects_source_unique (source)
);

-- SiteSettings module (migration 31)
CREATE TABLE IF NOT EXISTS site_settings (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, robots_txt TEXT, llms_txt TEXT, llms_config JSON, `schema` JSON, integrations JSON, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL);
-- ActivityLog module (migrations 32-33)
CREATE TABLE IF NOT EXISTS activity_logs (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, user_id CHAR(36) NULL, action VARCHAR(255) NOT NULL, subject_type VARCHAR(255), subject_id VARCHAR(255), description VARCHAR(255) NOT NULL,
  changes JSON, properties JSON, created_at TIMESTAMP NULL,
  INDEX activity_logs_user_id_index (user_id), INDEX activity_logs_subject_index (subject_type, subject_id), INDEX activity_logs_action_index (action), INDEX activity_logs_created_at_index (created_at)
);
CREATE TABLE IF NOT EXISTS activity_log_deletion_requests (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, year SMALLINT UNSIGNED NOT NULL, requested_by CHAR(36) NULL, token_hash VARCHAR(255) NOT NULL UNIQUE, status VARCHAR(255) NOT NULL DEFAULT 'pending',
  expires_at TIMESTAMP NOT NULL, confirmed_at TIMESTAMP NULL, executed_at TIMESTAMP NULL, deleted_count INT UNSIGNED, created_at TIMESTAMP NULL, updated_at TIMESTAMP NULL,
  INDEX activity_log_deletion_requests_status_year_index (status, year)
);
