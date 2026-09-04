ALTER TABLE app_extensions
    ADD CONSTRAINT app_extensions_type_check
    CHECK (type IN ('payment', 'shipping', 'custom'));

ALTER TABLE app_scopes
    ADD CONSTRAINT app_scopes_name_check
    CHECK (scope ~ '^(read|write)_[a-z_]+$');
