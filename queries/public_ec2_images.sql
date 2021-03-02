WITH image_access AS (
SELECT
  I.uri,
  snapshot_account_id(LP.value) AS account_id
FROM
  aws_ec2_image AS I
  cross join lateral jsonb_array_elements(I.launchpermissions) AS LP
)
SELECT
	IA.uri,
	bool_or(IA.account_id = '*') AS is_public,
	ARRAY_AGG(IA.account_id) FILTER (WHERE EXISTS (
		SELECT 1 FROM aws_organizations_account AS A
		WHERE A.id = IA.account_id AND $1 != A.id
	)) AS inorg,
	ARRAY_AGG(IA.account_id) FILTER (WHERE NOT EXISTS (
		SELECT 1 FROM aws_organizations_account AS A
		WHERE A.id = IA.account_id AND $1 != A.id
	)) AS external
FROM
	image_access AS IA
GROUP BY IA.uri