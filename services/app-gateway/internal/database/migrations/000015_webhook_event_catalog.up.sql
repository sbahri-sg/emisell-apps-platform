CREATE TABLE webhook_event_definitions (
    event text PRIMARY KEY CHECK (event ~ '^[a-z]+/[a-z_]+$'),
    name text NOT NULL CHECK (char_length(name) BETWEEN 2 AND 120),
    description text NOT NULL CHECK (char_length(description) BETWEEN 10 AND 500),
    source text NOT NULL CHECK (source IN ('app_platform', 'emisell_backend')),
    availability text NOT NULL CHECK (availability IN ('available', 'planned')),
    required_scope text,
    api_version text NOT NULL CHECK (api_version ~ '^20[0-9]{2}-[0-9]{2}-[0-9]{2}$'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO webhook_event_definitions (
    event, name, description, source, availability, required_scope, api_version
) VALUES
    ('app/uninstalled', 'App uninstalled', 'Sent after a merchant disconnects this app and its installation access is revoked.', 'app_platform', 'available', NULL, '2026-09-01'),
    ('products/created', 'Product created', 'Sent when a product is created in the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_products', '2026-09-01'),
    ('products/updated', 'Product updated', 'Sent when a product changes in the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_products', '2026-09-01'),
    ('products/deleted', 'Product deleted', 'Sent when a product is deleted from the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_products', '2026-09-01'),
    ('inventory/updated', 'Inventory updated', 'Sent when inventory availability changes for a connected merchant.', 'emisell_backend', 'planned', 'read_inventory', '2026-09-01'),
    ('orders/created', 'Order created', 'Sent when an order is created for the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_orders', '2026-09-01'),
    ('orders/updated', 'Order updated', 'Sent when an order changes for the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_orders', '2026-09-01'),
    ('orders/cancelled', 'Order cancelled', 'Sent when an order is cancelled for the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_orders', '2026-09-01'),
    ('customers/created', 'Customer created', 'Sent when a customer is created in the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_customers', '2026-09-01'),
    ('customers/updated', 'Customer updated', 'Sent when a customer profile changes in the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_customers', '2026-09-01'),
    ('customers/deleted', 'Customer deleted', 'Sent when a customer is deleted from the connected Emisell merchant account.', 'emisell_backend', 'planned', 'read_customers', '2026-09-01'),
    ('fulfillments/updated', 'Fulfillment updated', 'Sent when fulfillment, shipment, tracking, or delivery state changes.', 'emisell_backend', 'planned', 'read_fulfillments', '2026-09-01');

ALTER TABLE webhook_events
    ADD COLUMN source text NOT NULL DEFAULT 'test' CHECK (source IN ('app_platform', 'emisell_backend', 'test')),
    ADD COLUMN merchant_id text,
    ADD COLUMN installation_id uuid REFERENCES app_installations(id) ON DELETE SET NULL;

CREATE INDEX webhook_events_installation_created_idx
    ON webhook_events (installation_id, created_at DESC)
    WHERE installation_id IS NOT NULL;

COMMENT ON TABLE webhook_event_definitions IS 'Authoritative developer-facing webhook event catalog. Planned events cannot be subscribed until their producer contract is available.';
COMMENT ON COLUMN webhook_events.merchant_id IS 'Opaque Emisell merchant identifier copied into delivery context when the event belongs to an installation.';
