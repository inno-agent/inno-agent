-- Dev seed: review-consumer bot credentials.
-- Secret value must match REVIEW_SERVICE_CLIENT_SECRET in .env.
-- In production: insert manually with a strong randomly-generated secret.
INSERT INTO service_clients (client_id, secret_hash, name)
SELECT 'review-consumer',
       crypt('dev-review-secret-change-in-prod', gen_salt('bf', 10)),
       'PR Review Bot'
WHERE NOT EXISTS (
    SELECT 1 FROM service_clients WHERE client_id = 'review-consumer'
);
