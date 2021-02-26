SELECT
  I.end_date,
  P.name AS organization,
  O.arn
FROM
  import_job AS I
  INNER JOIN provider_account AS P
    ON I.provider_account_id = P.id
  INNER JOIN aws_organizations_organization AS O
    ON P.name = O.id
WHERE
  I.end_date IS NOT NULL
ORDER BY
  I.end_date DESC
LIMIT 1