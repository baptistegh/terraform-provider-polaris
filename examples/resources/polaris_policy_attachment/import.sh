# Import ID: {catalog}/{policy namespace}/{policy name}/{target type}[/{target path}],
# with namespace and target path levels joined by ".". Parameters are not imported.
terraform import polaris_policy_attachment.catalog lakehouse/governance/nightly-compaction/catalog
terraform import polaris_policy_attachment.sales lakehouse/governance/nightly-compaction/namespace/sales
terraform import polaris_policy_attachment.orders lakehouse/governance/nightly-compaction/table-like/sales.orders
