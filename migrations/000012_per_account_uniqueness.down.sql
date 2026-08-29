ALTER TABLE endpoints
    DROP CONSTRAINT endpoints_account_method_path_key;

ALTER TABLE endpoints
    ADD CONSTRAINT endpoints_method_path_key UNIQUE (method, path);

ALTER TABLE mock_resources
    DROP CONSTRAINT mock_resources_account_collection_resource_id_key;

ALTER TABLE mock_resources
    ADD CONSTRAINT mock_resources_collection_resource_id_key UNIQUE (collection, resource_id);