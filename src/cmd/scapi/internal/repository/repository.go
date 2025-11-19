package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/firstkb/sc-api/cmd/scapi/internal/utils"
	"github.com/firstkb/sc-api/internal/sqlserver"
)

type Repository struct {
	Client *sqlserver.Client
	Logger *slog.Logger
	Claim  *utils.Claim
}

func NewRepository(client *sqlserver.Client, logger *slog.Logger) Repository {
	return Repository{
		Client: client,
		Logger: logger,
	}
}

func (r *Repository) OpenDB(ctx context.Context) (*sqlserver.Database, error) {
	claim, err := utils.GetClaim(ctx)
	if err != nil {
		return nil, err
	}

	db, err := r.Client.OpenDB(ctx, claim.ClientID)
	if err != nil {
		return nil, err
	}

	r.Claim = claim

	return db, nil
}

func (r *Repository) GetPageFromId(ctx context.Context, pageID string) (*ExtDBpg, error) {

	query := `
	SELECT TOP(1) ExtDBpg_id, ExtDBpg_ExtDBtbl_id, ExtDBpg_title, ExtDBpg_act, 
	ISNULL(ExtDBpg_webstatus,     '') AS ExtDBpg_webstatus,
    ISNULL(ExtDBpg_webstatusnew,  '') AS ExtDBpg_webstatusnew,
    ISNULL(ExtDBpg_webstatusdone, '') AS ExtDBpg_webstatusdone, 
	ExtDBpg_aEdit, ExtDBpg_aNew, ExtDBpg_files, 
	ISNULL(ExtDBpg_webdate,       '') AS ExtDBpg_webdate,
	ISNULL(ExtDBpg_webby,         '') AS ExtDBpg_webby,
    ISNULL(ExtDBpg_filter,        '') AS ExtDBpg_filter,
    ISNULL(ExtDBpg_order1,        '') AS ExtDBpg_order1,
    ISNULL(ExtDBpg_order2,        '') AS ExtDBpg_order2,
	ExtDBtbl_formtype, ExtDBtbl_type, ExtDBtbl_title,
	ISNULL(ExtDBpg_capage,        '') AS ExtDBpg_capage 
	FROM [{db}].dbo.[ExtDBpg]
	LEFT JOIN [{db}].dbo.[ExtDBtbl] on ExtDBtbl.ExtDBtbl_id=ExtDBpg.ExtDBpg_ExtDBtbl_id
	WHERE ExtDBpg_id = @p1
	`

	db, err := r.OpenDB(ctx)
	if err != nil {
		return nil, err
	}

	row := db.QueryRow(query, pageID)

	var ExtDBpg ExtDBpg
	err = row.Scan(
		&ExtDBpg.ID,
		&ExtDBpg.TableID,
		&ExtDBpg.Title,
		&ExtDBpg.Active,
		&ExtDBpg.WebStatus,
		&ExtDBpg.WebStatusNew,
		&ExtDBpg.WebStatusFinish,
		&ExtDBpg.AEdit,
		&ExtDBpg.ANew,
		&ExtDBpg.Files,
		&ExtDBpg.WebDate,
		&ExtDBpg.WebBy,
		&ExtDBpg.Filter,
		&ExtDBpg.Order1,
		&ExtDBpg.Order2,
		&ExtDBpg.TblFormType,
		&ExtDBpg.TblType,
		&ExtDBpg.TblTitle,
		&ExtDBpg.CaPage,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ExtDBpg not found")
		}
		return nil, err
	}

	return &ExtDBpg, nil
}

func (r *Repository) GetTableSchemaEzData(ctx context.Context, tableID int) (map[string]string, error) {
	schemaSQL := fmt.Sprintf(`
        SELECT TOP 1 *
        FROM [{db}].dbo.ExtDB%d
        WHERE 1=0
    `, tableID)

	db, err := r.OpenDB(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(schemaSQL)
	if err != nil {
		return nil, fmt.Errorf("schema query error: %w", err)
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("schema column types error: %w", err)
	}

	schemaMap := make(map[string]string)
	for _, ct := range columnTypes {
		colName := ct.Name()
		schemaMap[colName] = ct.DatabaseTypeName()
	}

	return schemaMap, nil
}

func (r *Repository) GetTableSchema(ctx context.Context, tableName string) (map[string]string, error) {
	schemaSQL := fmt.Sprintf(`
        SELECT TOP 1 *
        FROM [{db}].dbo.%s
        WHERE 1=0
    `, tableName)

	db, err := r.OpenDB(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(schemaSQL)
	if err != nil {
		return nil, fmt.Errorf("schema query error: %w", err)
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("schema column types error: %w", err)
	}

	schemaMap := make(map[string]string)
	for _, ct := range columnTypes {
		colName := ct.Name()
		schemaMap[colName] = ct.DatabaseTypeName()
	}

	return schemaMap, nil
}

func (r *Repository) GetPlaseholder(ctx context.Context, insertFields []string) string {
	placeholders := make([]string, len(insertFields))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("@p%d", i+1)
	}
	placeholdersJoined := strings.Join(placeholders, ", ")

	return placeholdersJoined
}
