WITH snapshot_access AS (
SELECT
  S.uri,
  snapshot_account_id(CVP.value) AS account_id
FROM
  aws_ec2_snapshot AS S
  cross join lateral jsonb_array_elements(S.createvolumepermissions) AS CVP
)
SELECT
	SA.uri,
	bool_or(SA.account_id = '*') AS is_public,
	ARRAY_AGG(SA.account_id) FILTER (WHERE EXISTS (
		SELECT 1 FROM aws_organizations_account AS A
		WHERE A.id = SA.account_id AND arn_account_id(SA.uri) != A.id
	)) AS inorg,
	ARRAY_AGG(SA.account_id) FILTER (WHERE NOT EXISTS (
		SELECT 1 FROM aws_organizations_account AS A
		WHERE A.id = SA.account_id AND arn_account_id(SA.uri) != A.id
	)) AS external
FROM
	snapshot_access AS SA
GROUP BY SA.uri