// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package apiGatewayConfDeploy

import (
	"database/sql"
	"sync"

	"github.com/apid/apid-core"
	"reflect"
)

var (
	gwBlobId int64
)

type Configuration struct {
	ID             string
	OrgID          string
	EnvID          string
	BlobID         string
	BlobResourceID string
	Type           string
	Name           string
	Revision       string
	Path           string
	Created        string
	CreatedBy      string
	Updated        string
	UpdatedBy      string
}

type SQLExec interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type dbManagerInterface interface {
	setDbVersion(string)
	initDb() error
	getUnreadyBlobs() ([]string, error)
	getReadyDeployments(typeFilter string) ([]Configuration, error)
	updateLocalFsLocation(string, string) error
	getLocalFSLocation(string) (string, error)
	getConfigById(string) (*Configuration, error)
	getLastSequence() (string, error)
}

type dbManager struct {
	data  apid.DataService
	db    apid.DB
	dbMux sync.RWMutex
}

func (dbc *dbManager) setDbVersion(version string) {
	db, err := dbc.data.DBVersion(version)
	if err != nil {
		log.Panicf("Unable to access database: %v", err)
	}
	dbc.dbMux.Lock()
	dbc.db = db
	dbc.dbMux.Unlock()
}

func (dbc *dbManager) getDb() apid.DB {
	dbc.dbMux.RLock()
	defer dbc.dbMux.RUnlock()
	return dbc.db
}

func (dbc *dbManager) initDb() error {
	tx, err := dbc.getDb().Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
	CREATE TABLE IF NOT EXISTS apid_blob_available (
		id text primary key,
   		local_fs_location text NOT NULL
	);
	`)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	log.Debug("Database table apid_blob_available created.")
	return nil
}

func (dbc *dbManager) getConfigById(id string) (config *Configuration, err error) {
	row := dbc.getDb().QueryRow(`
	SELECT 	a.id,
			a.organization_id,
			a.environment_id,
			a.bean_blob_id,
			a.resource_blob_id,
			a.type,
			a.name,
			a.revision,
			a.path,
			a.created_at,
			a.created_by,
			a.updated_at,
			a.updated_by
		FROM metadata_runtime_entity_metadata as a
		WHERE a.id = ?;
	`, id)
	config, err = dataDeploymentsFromRow(row)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// getUnreadyDeployments() returns array of resources that are not yet to be processed
func (dbc *dbManager) getUnreadyBlobs() (ids []string, err error) {

	rows, err := dbc.getDb().Query(`
	SELECT id FROM (
			SELECT a.bean_blob_id as id
			FROM metadata_runtime_entity_metadata as a
			WHERE a.bean_blob_id NOT IN
			(SELECT b.id FROM apid_blob_available as b)
		UNION
			SELECT a.resource_blob_id as id
			FROM metadata_runtime_entity_metadata as a
			WHERE a.resource_blob_id NOT IN
			(SELECT b.id FROM apid_blob_available as b)
	)
	WHERE id IS NOT NULL AND id != ''
	;
	`)
	if err != nil {
		log.Errorf("DB Query for project_runtime_blob_metadata failed %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return
		}
		ids = append(ids, id)
	}

	log.Debugf("Unready blobId %v", ids)
	return
}

func (dbc *dbManager) getReadyDeployments(typeFilter string) ([]Configuration, error) {

	// An alternative statement is in get_ready_deployments.sql
	// Need testing with large data volume to determine which is better

	var rows *sql.Rows
	var err error
	if typeFilter == "" {
		rows, err = dbc.getDb().Query(`
		SELECT 	a.id,
			a.organization_id,
			a.environment_id,
			a.bean_blob_id,
			a.resource_blob_id,
			a.type,
			a.name,
			a.revision,
			a.path,
			a.created_at,
			a.created_by,
			a.updated_at,
			a.updated_by
		FROM metadata_runtime_entity_metadata as a
		WHERE a.id IN (
			SELECT
					a.id
				FROM metadata_runtime_entity_metadata as a
				INNER JOIN apid_blob_available as b
				ON a.resource_blob_id = b.id
				WHERE a.resource_blob_id IS NOT NULL AND a.resource_blob_id != ""
			INTERSECT
				SELECT
					a.id
				FROM metadata_runtime_entity_metadata as a
				INNER JOIN apid_blob_available as b
				ON a.bean_blob_id = b.id
				WHERE a.resource_blob_id IS NOT NULL AND a.resource_blob_id != ""

			UNION
				SELECT
					a.id
				FROM metadata_runtime_entity_metadata as a
				INNER JOIN apid_blob_available as b
				ON a.bean_blob_id = b.id
				WHERE a.resource_blob_id IS NULL OR a.resource_blob_id = ""
		)
	;
	`)
	} else {
		rows, err = dbc.getDb().Query(`
		SELECT 	a.id,
			a.organization_id,
			a.environment_id,
			a.bean_blob_id,
			a.resource_blob_id,
			a.type,
			a.name,
			a.revision,
			a.path,
			a.created_at,
			a.created_by,
			a.updated_at,
			a.updated_by
		FROM metadata_runtime_entity_metadata as a
		WHERE a.type = ?
		AND a.id IN (
			SELECT
					a.id
				FROM metadata_runtime_entity_metadata as a
				INNER JOIN apid_blob_available as b
				ON a.resource_blob_id = b.id
				WHERE a.resource_blob_id IS NOT NULL AND a.resource_blob_id != ""
			INTERSECT
				SELECT
					a.id
				FROM metadata_runtime_entity_metadata as a
				INNER JOIN apid_blob_available as b
				ON a.bean_blob_id = b.id
				WHERE a.resource_blob_id IS NOT NULL AND a.resource_blob_id != ""

			UNION
				SELECT
					a.id
				FROM metadata_runtime_entity_metadata as a
				INNER JOIN apid_blob_available as b
				ON a.bean_blob_id = b.id
				WHERE a.resource_blob_id IS NULL OR a.resource_blob_id = ""
		)
	;
	`, typeFilter)
	}

	if err != nil {
		log.Errorf("DB Query for project_runtime_blob_metadata failed %v", err)
		return nil, err
	}
	defer rows.Close()

	deployments, err := dataDeploymentsFromRows(rows)
	if err != nil {
		return nil, err
	}

	log.Debugf("Configurations ready: %v", deployments)

	return deployments, nil

}

func (dbc *dbManager) updateLocalFsLocation(blobId, localFsLocation string) error {
	txn, err := dbc.getDb().Begin()
	if err != nil {
		return err
	}
	defer txn.Rollback()
	_, err = txn.Exec(`
		INSERT OR IGNORE INTO apid_blob_available (
		id,
		local_fs_location
		) VALUES (?, ?);`, blobId, localFsLocation)
	if err != nil {
		log.Errorf("INSERT apid_blob_available id {%s} local_fs_location {%s} failed", localFsLocation, err)
		return err
	}
	err = txn.Commit()
	if err != nil {
		log.Errorf("UPDATE apid_blob_available id {%s} local_fs_location {%s} failed", localFsLocation, err)
		return err
	}

	log.Debugf("INSERT apid_blob_available {%s} local_fs_location {%s} succeeded", blobId, localFsLocation)
	return nil

}

func (dbc *dbManager) getLocalFSLocation(blobId string) (localFsLocation string, err error) {

	log.Debugf("Getting the blob file for blobId {%s}", blobId)
	rows, err := dbc.getDb().Query("SELECT local_fs_location FROM apid_blob_available WHERE id = '" + blobId + "'")
	if err != nil {
		log.Errorf("SELECT local_fs_location failed %v", err)
		return "", err
	}

	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&localFsLocation)
		if err != nil {
			log.Errorf("Scan local_fs_location failed %v", err)
			return "", err
		}
		log.Debugf("Got the blob file {%s} for blobId {%s}", localFsLocation, blobId)
	}
	return
}

func (dbc *dbManager) getLastSequence() (string, error) {
	var lastSequence sql.NullString
	err := dbc.getDb().QueryRow("select last_sequence from EDGEX_APID_CLUSTER LIMIT 1").Scan(&lastSequence)
	if err != nil && err != sql.ErrNoRows {
		log.Errorf("Failed to select last_sequence from EDGEX_APID_CLUSTER: %v", err)
		return "", err
	}
	ret := ""
	if lastSequence.Valid {
		ret = lastSequence.String
	}
	log.Debugf("lastSequence: %s", lastSequence)
	return ret, nil
}

func dataDeploymentsFromRows(rows *sql.Rows) ([]Configuration, error) {
	tmp, err := structFromRows(reflect.TypeOf((*Configuration)(nil)).Elem(), rows)
	if err != nil {
		return nil, err
	}
	return tmp.([]Configuration), nil
}

func structFromRows(t reflect.Type, rows *sql.Rows) (interface{}, error) {
	num := t.NumField()
	cols := make([]interface{}, num)
	slice := reflect.New(reflect.SliceOf(t)).Elem()
	for i := range cols {
		cols[i] = new(sql.NullString)
	}
	for rows.Next() {
		v := reflect.New(t).Elem()
		err := rows.Scan(cols...)
		if err != nil {
			return nil, err
		}
		for i := range cols {
			p := cols[i].(*sql.NullString)
			if p.Valid {
				v.Field(i).SetString(p.String)
			}
		}
		slice = reflect.Append(slice, v)
	}
	return slice.Interface(), nil
}

func dataDeploymentsFromRow(row *sql.Row) (*Configuration, error) {
	tmp, err := structFromRow(reflect.TypeOf((*Configuration)(nil)).Elem(), row)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Errorf("Error in dataDeploymentsFromRow: %v", err)
		}
		return nil, err
	}
	config := tmp.(Configuration)
	return &config, nil
}

func structFromRow(t reflect.Type, row *sql.Row) (interface{}, error) {
	num := t.NumField()
	cols := make([]interface{}, num)
	for i := range cols {
		cols[i] = new(sql.NullString)
	}
	v := reflect.New(t).Elem()
	err := row.Scan(cols...)
	if err != nil {
		return nil, err
	}
	for i := range cols {
		p := cols[i].(*sql.NullString)
		if p.Valid {
			v.Field(i).SetString(p.String)
		}
	}
	return v.Interface(), nil
}
