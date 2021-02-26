package report

import (
	"database/sql"
	"io/ioutil"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

type Row struct {
	Arn              string
	Service          string
	ProviderType     string
	InOrgAccounts    []string
	ExternalAccounts []string
	IsPublic         bool
}

func (r *Row) Access() string {
	if r.IsPublic {
		return "Public"
	}
	if len(r.ExternalAccounts) > 0 {
		return "External Accounts"
	}
	if len(r.InOrgAccounts) > 0 {
		return "In-Org Accounts"
	}
	return "Private"
}

// Metadata includes information about the report, such as when the data was
// snapshotted and for what account
type Metadata struct {
	Imported     time.Time
	Generated    time.Time
	Account      string
	Organization string
}

type Report struct {
	Metadata *Metadata
	Rows     []Row
}

// Generate uses a connection string to postgres and a list of designated-safe ports
// to produce a report assessing the risk of each resource policy that has been imported.
func Generate(connectionString string) (*Report, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to connect to db")
	}
	defer db.Close()
	err = db.Ping()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to ping db")
	}
	log.Info("db ready")
	err = installDbFunctions(db)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to install fixture functions")
	}
	rows, err := runResourceAccessQuery(db)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run analysis query")
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return sortRowsLess(&rows[i], &rows[j])
	})
	metadata, err := loadMetadata(db)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load metadata")
	}
	return &Report{
		Rows:     rows,
		Metadata: metadata,
	}, nil
}

var statusIndex map[string]int = map[string]int{
	"Public":            0,
	"External Accounts": 1,
	"In-Org Accounts":   2,
	"Private":           3,
}

func arnRegion(arn string) string {
	parts := strings.Split(arn, ":")
	return parts[3]
}

func loadMetadata(db *sql.DB) (*Metadata, error) {
	query, err := loadQuery("most_recent_import")
	if err != nil {
		return nil, errors.Wrap(err, "failed to load query")
	}
	queryRows, err := db.Query(query)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to query for most recent import")
	}
	defer queryRows.Close()
	if !queryRows.Next() {
		return nil, errors.New("Query for most recent import job found no results")
	}
	var endDate time.Time
	var organization string
	var arn string
	err = queryRows.Scan(&endDate, &organization, &arn)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read most recent import job row")
	}
	parts := strings.Split(arn, ":")
	accountID := parts[4]
	if strings.HasPrefix(organization, "OrgDummy") {
		organization = "<NONE>"
	}
	return &Metadata{
		Imported:     endDate,
		Generated:    time.Now(),
		Account:      accountID,
		Organization: organization,
	}, nil
}

// Sort by status first, then region, then name
func sortRowsLess(a, b *Row) bool {
	if a.Access() == b.Access() {
		return a.Arn < b.Arn
	}
	aIndex := statusIndex[a.Access()]
	bIndex := statusIndex[b.Access()]
	return aIndex < bIndex
}

func installDbFunctions(db *sql.DB) error {
	functions, err := loadQuery("functions")
	if err != nil {
		return errors.New("Failed to load sql for helper functions")
	}
	result, err := db.Exec(functions)
	if err != nil {
		return err
	}
	log.Infof("result %v", result)
	return nil
}

func runResourceAccessQuery(db *sql.DB) ([]Row, error) {
	analysisQuery, err := loadQuery("resource_policy_exposure")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to to load analysis query")
	}
	rows, err := db.Query(analysisQuery)
	if err != nil {
		return nil, errors.Wrap(err, "DB error analyzing")
	}
	defer rows.Close()
	results := make([]Row, 0)
	for rows.Next() {
		row := Row{}
		err = rows.Scan(&row.Arn, &row.Service, &row.ProviderType,
			pq.Array(&row.InOrgAccounts), pq.Array(&row.ExternalAccounts),
			&row.IsPublic)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshal a row")
		}
		results = append(results, row)
	}
	log.Infof("rows %v", len(results))
	return results, nil
}

func loadQuery(name string) (string, error) {
	filename := "/queries/" + name + ".sql"
	f, err := pkger.Open(filename)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to open %v", filename)
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to read %v", filename)
	}
	return string(bytes), nil
}
