-- given '*' or an arn, returns '*' or the account id
CREATE OR REPLACE FUNCTION arn_account_id(arn TEXT)
RETURNS TEXT AS $$
	SELECT
		CASE
			WHEN arn = '*' THEN '*'
			ELSE split_part(arn, ':', 5)
		END
$$ LANGUAGE sql IMMUTABLE STRICT;

-- Given a snapshot permission, return an account id or '*'
CREATE OR REPLACE FUNCTION snapshot_account_id(perm JSONB)
RETURNS TEXT AS $$
  SELECT
    CASE
      WHEN lower(I.id) = 'all' THEN '*'
      ELSE I.id
    END AS account_id
  FROM
    ( SELECT COALESCE(perm ->> 'Group', perm ->> 'UserId') AS id ) AS I
$$ LANGUAGE sql IMMUTABLE STRICT;

-- marked stable because we may join with accounts table
CREATE OR REPLACE FUNCTION extract_account_ids(inval TEXT)
RETURNS Table(account_id TEXT) AS $$
  SELECT
		CASE
			WHEN inval = '*' THEN '*'
			WHEN inval LIKE 'arn:%' THEN arn_account_id(inval)
			ELSE inval
		END
$$ LANGUAGE sql STABLE STRICT;

-- marked stable because it calls a stable function
CREATE OR REPLACE FUNCTION condition_allowed_accounts(condition JSONB)
RETURNS Table(account_id TEXT) AS $$
	SELECT COALESCE(
	(SELECT
		A.account_id
	FROM
		jsonb_each(condition -> 'StringEquals') AS SE
		CROSS JOIN LATERAL unpack_maybe_array(SE.value) AS ConditionValue
    CROSS JOIN LATERAL extract_account_ids(ConditionValue.value #>> '{}') AS A
	WHERE
		lower(SE.key) IN ('kms:calleraccount', 'aws:sourceowner', 'aws:principalaccount', 'aws:principalarn', 'aws:sourceaccount', 'aws:sourcearn'))
	, '*')
$$ LANGUAGE sql STABLE STRICT;

CREATE OR REPLACE FUNCTION allowed_account_ids(S JSONB)
RETURNS Table(account_id TEXT)  AS $$
  SELECT
    arn_account_id(P.value #>> '{}')  AS account_id
  FROM
    jsonb_array_elements(S -> 'Principal' -> 'AWS') AS P
  WHERE
    S ->> 'Effect' = 'Allow'
$$ LANGUAGE sql IMMUTABLE STRICT;