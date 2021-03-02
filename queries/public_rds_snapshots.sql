WITH snapshot_access AS (
SELECT
  S.uri,
	all_to_star(CVP.value) AS account_id
FROM
  aws_rds_dbsnapshot AS S
  cross join lateral jsonb_array_elements(S.restore) AS CVP
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