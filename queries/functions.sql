-- given '*' or an arn, returns '*' or the account id
CREATE OR REPLACE FUNCTION arn_account_id(arn TEXT)
RETURNS TEXT AS $$
	SELECT
		CASE
			WHEN arn = '*' THEN '*'
			ELSE split_part(arn, ':', 5)
		END
$$ LANGUAGE sql IMMUTABLE STRICT;

CREATE OR REPLACE FUNCTION all_to_star(identifier TEXT)
RETURNS TEXT AS $$
SELECT
    CASE
      WHEN lower(identifier) = 'all' THEN '*'
      ELSE identifier
    END
$$ LANGUAGE sql IMMUTABLE STRICT;

CREATE OR REPLACE FUNCTION all_to_star(identifier JSONB)
RETURNS TEXT AS $$
SELECT
    CASE
      WHEN lower(identifier #>> '{}') = 'all' THEN '*'
      ELSE identifier #>> '{}'
    END
$$ LANGUAGE sql IMMUTABLE STRICT;

-- Given a snapshot permission, return an account id or '*'
CREATE OR REPLACE FUNCTION snapshot_account_id(perm JSONB)
RETURNS TEXT AS $$
  SELECT
		all_to_star(I.id)
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
  SELECT
    COALESCE(
      (
        SELECT
          A.account_id
        FROM
          (
            SELECT
              SE.*
            FROM
              jsonb_each(condition -> 'StringEquals') AS SE
            WHERE
              lower(SE.key) IN ('kms:calleraccount', 'aws:sourceowner', 'aws:principalaccount', 'aws:principalarn', 'aws:sourceaccount', 'aws:sourcearn')
            UNION
            SELECT
              AE.*
            FROM
              jsonb_each(condition -> 'ArnEquals') AS AE
            WHERE
              lower(AE.key) IN ('aws:principalarn', 'aws:sourcearn')
          ) AS Identifier
          CROSS JOIN LATERAL unpack_maybe_array(Identifier.value) AS ConditionValue
          CROSS JOIN LATERAL extract_account_ids(ConditionValue.value #>> '{}') AS A
      ),
      '*'
    )
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