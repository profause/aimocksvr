ALTER TABLE endpoints
    DROP CONSTRAINT endpoints_method_path_key;

ALTER TABLE endpoints
    ADD CONSTRAINT endpoints_account_method_path_key UNIQUE (account_id, method, path);

ALTER TABLE mock_resources
    DROP CONSTRAINT mock_resources_collection_resource_id_key;

ALTER TABLE mock_resources
    ADD CONSTRAINT mock_resources_account_collection_resource_id_key UNIQUE (account_id, collection, resource_id);