WITH statement_access AS (
-- for each resource, what accounts have any kind of access
SELECT
	RA.resource_id,
	CASE
		WHEN A.account_id = '*' AND CA.account_id = '*' THEN '*'
		WHEN A.account_id = '*' THEN CA.account_id
		ELSE A.account_id
	END AS account_id
FROM
	resource_attribute AS RA
	CROSS JOIN LATERAL jsonb_array_elements(RA.attr_value -> 'Statement') AS S
	CROSS JOIN LATERAL allowed_account_ids(S.value) AS A
	CROSS JOIN LATERAL condition_allowed_accounts(COALESCE(S.value -> 'Condition', '{}'::jsonb)) AS CA
WHERE
	RA.type = 'Metadata'
	AND RA.attr_name = 'Policy'
	AND (
		A.account_id = '*'
		OR CA.account_id = '*'
		OR A.account_id = CA.account_id
	)
), external_accounts AS (
-- given account access to a resource, filter to just the external accounts
SELECT
	SA.resource_id,
	SA.account_id
FROM
	statement_access AS SA
	INNER JOIN resource AS R
		ON R.id = SA.resource_id
WHERE
	arn_account_id(R.uri) != SA.account_id
	AND NOT EXISTS (SELECT 1 FROM aws_organizations_account AS A WHERE A.Id = SA.account_id)
	AND SA.account_id != '*'
GROUP BY SA.resource_id, SA.account_id
), inorg_accounts AS (
SELECT
	SA.resource_id,
	SA.account_id
FROM
	statement_access AS SA
	INNER JOIN resource AS R
		ON R.id = SA.resource_id
WHERE
	arn_account_id(R.uri) != SA.account_id
	AND EXISTS (SELECT 1 FROM aws_organizations_account AS A WHERE A.Id = SA.account_id)
	AND SA.account_id != '*'
GROUP BY SA.resource_id, SA.account_id
), resource_ids AS (
SELECT
	DISTINCT(resource_id)
FROM statement_access
), account_lists AS (
SELECT
	RID.resource_id,
	ARRAY_AGG(IA.account_id) FILTER (WHERE IA.account_id IS NOT NULL) AS inorg,
	ARRAY_AGG(EA.account_id) FILTER (WHERE EA.account_id IS NOT NULL) AS external
FROM
	resource_ids AS RID
	LEFT JOIN inorg_accounts AS IA
		ON IA.resource_id = RID.resource_id
	LEFT JOIN external_accounts AS EA
		ON EA.resource_id = RID.resource_id
GROUP BY RID.resource_id
), public_resources AS (
SELECT
	SA.resource_id,
	bool_or(SA.account_id = '*') AS is_public
FROM
	statement_access AS SA
GROUP BY SA.resource_id
)
SELECT
	R.uri,
	R.service,
	R.provider_type,
	AL.inorg,
	AL.external,
	PR.is_public
FROM
	resource AS R
	INNER JOIN account_lists AS AL
		ON AL.resource_id = R.id
	INNER JOIN public_resources AS PR
		ON PR.resource_id = R.id
