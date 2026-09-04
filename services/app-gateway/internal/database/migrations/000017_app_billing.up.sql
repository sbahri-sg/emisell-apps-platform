-- Deliberately no data backfill, seed prices, payment jobs, or existing-plan changes.
ALTER TABLE app_installations ADD CONSTRAINT app_installations_billing_identity UNIQUE(id,merchant_id,environment);
CREATE TABLE app_plans (
    id uuid PRIMARY KEY,
    app_id uuid NOT NULL REFERENCES apps(id),
    request_key text NOT NULL,
    request_hash text NOT NULL,
    document jsonb NOT NULL,
    UNIQUE(app_id, request_key),
    UNIQUE(id, app_id),
    CHECK (document->>'id' = id::text AND document->>'appId' = app_id::text),
    CHECK (document->>'status' IN ('active','archived')),
    CHECK ((document->>'amountMinor')::bigint BETWEEN 0 AND 1000000000000),
    CHECK (document->>'currency' IN ('IDR','USD')),
    CHECK ((document->>'interval' = 'free' AND (document->>'amountMinor')::bigint = 0)
        OR (document->>'interval' = 'monthly' AND (document->>'amountMinor')::bigint > 0))
);

CREATE TABLE merchant_app_billing (
    merchant_id text NOT NULL,
    environment text NOT NULL CHECK (environment IN ('sandbox','production')),
    account jsonb,
    PRIMARY KEY(merchant_id, environment)
);

-- JSONB documents retain the exact consent, price and invoice snapshots.
-- Relational selectors, foreign keys and unique indexes enforce isolation.
CREATE TABLE app_billing_quotes (
    id uuid PRIMARY KEY,
    merchant_id text NOT NULL,
    environment text NOT NULL,
    installation_id uuid NOT NULL REFERENCES app_installations(id),
    document jsonb NOT NULL,
    FOREIGN KEY(merchant_id, environment) REFERENCES merchant_app_billing,
    FOREIGN KEY(installation_id,merchant_id,environment) REFERENCES app_installations(id,merchant_id,environment),
    UNIQUE(id,installation_id,merchant_id,environment),
    CHECK (document->>'id' = id::text AND document->>'installationId' = installation_id::text)
);
CREATE TABLE app_subscriptions (
    id uuid PRIMARY KEY,
    merchant_id text NOT NULL,
    environment text NOT NULL,
    installation_id uuid NOT NULL REFERENCES app_installations(id),
    quote_id uuid NOT NULL UNIQUE REFERENCES app_billing_quotes(id),
    document jsonb NOT NULL,
    FOREIGN KEY(merchant_id, environment) REFERENCES merchant_app_billing,
    FOREIGN KEY(installation_id,merchant_id,environment) REFERENCES app_installations(id,merchant_id,environment),
    FOREIGN KEY(quote_id,installation_id,merchant_id,environment) REFERENCES app_billing_quotes(id,installation_id,merchant_id,environment),
    UNIQUE(id,merchant_id,environment),
    CHECK (document->>'id' = id::text AND document->>'installationId' = installation_id::text AND document->>'quoteId' = quote_id::text),
    CHECK (document->>'status' IN ('active','pending_payment','past_due','cancelled'))
);
CREATE UNIQUE INDEX one_live_app_subscription ON app_subscriptions(installation_id) WHERE document->>'status' <> 'cancelled';
CREATE INDEX app_subscriptions_merchant ON app_subscriptions(merchant_id, environment);
CREATE TABLE app_billing_invoices (
    merchant_id text NOT NULL,
    environment text NOT NULL,
    invoice_id text NOT NULL,
    document jsonb NOT NULL,
    PRIMARY KEY(merchant_id, environment, invoice_id),
    FOREIGN KEY(merchant_id, environment) REFERENCES merchant_app_billing,
    CHECK (document->>'invoiceId' = invoice_id AND document->>'merchantId' = merchant_id AND document->>'environment' = environment),
    CHECK (document->>'status' IN ('issued','paid','failed'))
);
CREATE TABLE app_subscription_charges (
    id uuid PRIMARY KEY,
    merchant_id text NOT NULL,
    environment text NOT NULL,
    subscription_id uuid NOT NULL REFERENCES app_subscriptions(id),
    period_start timestamptz NOT NULL,
    invoice_id text,
    document jsonb NOT NULL,
    FOREIGN KEY(merchant_id, environment, invoice_id) REFERENCES app_billing_invoices,
    FOREIGN KEY(subscription_id,merchant_id,environment) REFERENCES app_subscriptions(id,merchant_id,environment),
    UNIQUE(subscription_id, period_start),
    CHECK (document->>'id' = id::text AND document->>'subscriptionId' = subscription_id::text),
    CHECK (document->>'status' IN ('unbilled','invoiced','paid','failed','void'))
);
CREATE INDEX app_charges_merchant ON app_subscription_charges(merchant_id, environment);
CREATE TABLE app_billing_events (
    merchant_id text NOT NULL,
    environment text NOT NULL,
    event_id text NOT NULL,
    document jsonb NOT NULL,
    PRIMARY KEY(merchant_id, environment, event_id),
    FOREIGN KEY(merchant_id, environment) REFERENCES merchant_app_billing
);

-- Every uninstall path (developer, merchant or operator) cancels renewal in the
-- same transaction. Issued invoices remain payable; unbilled charges are voided.
CREATE FUNCTION cancel_uninstalled_app_billing() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status = 'uninstalled' AND OLD.status <> 'uninstalled' THEN
        UPDATE app_subscriptions SET document = document || jsonb_build_object(
            'status','cancelled','cancelledAt',to_jsonb(now()),'cancellationReason','uninstalled')
        WHERE installation_id = NEW.id AND document->>'status' <> 'cancelled';
        UPDATE app_subscription_charges SET document = document || '{"status":"void"}'::jsonb
        WHERE subscription_id IN (SELECT id FROM app_subscriptions WHERE installation_id = NEW.id)
          AND document->>'status' = 'unbilled';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER cancel_app_billing_on_uninstall AFTER UPDATE OF status ON app_installations
FOR EACH ROW EXECUTE FUNCTION cancel_uninstalled_app_billing();
