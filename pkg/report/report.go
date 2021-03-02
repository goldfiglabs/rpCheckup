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

// Access returns a human-readable string describing who can
// access the resource associated with this Row
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

// Generate uses a connection string to postgres to produce a report
// assessing the risk of each resource policy that has been imported.
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
	err = installDbFunctions(db)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to install fixture functions")
	}
	metadata, err := loadMetadata(db)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to load metadata")
	}
	rows, err := runResourceAccessQuery(db, metadata.Account)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run analysis query")
	}
	volumeSnapshotsRows, err := runEC2SnapshotQuery(db, metadata.Account)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run snapshot query")
	}
	rows = append(rows, volumeSnapshotsRows...)
	imageRows, err := runEC2ImageQuery(db, metadata.Account)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run snapshot query")
	}
	rows = append(rows, imageRows...)
	dbSnapshotsRows, err := runRDSDBSnapshotQuery(db, metadata.Account)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run snapshot query")
	}
	rows = append(rows, dbSnapshotsRows...)
	dbClusterSnapshotsRows, err := runRDSDBClusterSnapshotQuery(db, metadata.Account)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to run snapshot query")
	}
	rows = append(rows, dbClusterSnapshotsRows...)
	sort.SliceStable(rows, func(i, j int) bool {
		return sortRowsLess(&rows[i], &rows[j])
	})
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
	generated := time.Now()
	imported := endDate.In(generated.Location())
	return &Metadata{
		Imported:     imported,
		Generated:    generated,
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
	_, err = db.Exec(functions)
	if err != nil {
		return err
	}
	return nil
}

func runResourceAccessQuery(db *sql.DB, accountID string) ([]Row, error) {
	analysisQuery, err := loadQuery("resource_policy_exposure")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to to load analysis query")
	}
	rows, err := db.Query(analysisQuery, accountID)
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
	log.Debugf("%v result rows", len(results))
	return results, nil
}

func runSnapshotQuery(db *sql.DB, queryName string, service string, resource string, accountID string) ([]Row, error) {
	snapshotQuery, err := loadQuery(queryName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to load %v %v query", service, resource)
	}
	rows, err := db.Query(snapshotQuery, accountID)
	if err != nil {
		return nil, errors.Wrapf(err, "DB error analyzing %v %vs", service, resource)
	}
	defer rows.Close()
	results := []Row{}
	for rows.Next() {
		row := Row{
			Service:      service,
			ProviderType: resource,
		}
		err = rows.Scan(&row.Arn, &row.IsPublic, pq.Array(&row.InOrgAccounts),
			pq.Array(&row.ExternalAccounts))
		if err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshall a row")
		}
		results = append(results, row)
	}
	return results, nil
}

func runEC2SnapshotQuery(db *sql.DB, accountID string) ([]Row, error) {
	return runSnapshotQuery(db, "public_ec2_snapshots", "ec2", "Snapshot", accountID)
}

func runEC2ImageQuery(db *sql.DB, accountID string) ([]Row, error) {
	return runSnapshotQuery(db, "public_ec2_images", "ec2", "Image", accountID)
}

func runRDSDBClusterSnapshotQuery(db *sql.DB, accountID string) ([]Row, error) {
	return runSnapshotQuery(db, "public_rds_cluster_snapshots", "rds", "DBClusterSnapshot", accountID)
}

func runRDSDBSnapshotQuery(db *sql.DB, accountID string) ([]Row, error) {
	return runSnapshotQuery(db, "public_rds_snapshots", "rds", "DBSnapshot", accountID)
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
