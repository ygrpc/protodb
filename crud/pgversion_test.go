package crud

import (
	"database/sql"
	"reflect"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestSearchPgVersionInDB(t *testing.T) {
	type args struct {
		dburl string
	}
	tests := []struct {
		name        string
		args        args
		wantVersion TpgVersion
		wantErr     bool
	}{
		// TODO: Add test cases.
		{
			name: "10.20.30.5:9008",
			args: args{
				dburl: "postgres://postgres:20070902@10.20.30.5:9008/postgres?sslmode=disable",
			},
			wantVersion: TpgVersion{
				Major: 12,
				Minor: 15,
			},
			wantErr: false,
		},

		{
			name: "10.20.30.1:5432",
			args: args{
				dburl: "postgres://postgres:20070902@10.20.30.1:5432/postgres?sslmode=disable",
			},
			wantVersion: TpgVersion{
				Major: 17,
				Minor: 5,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := sql.Open("pgx", tt.args.dburl)
			if err != nil {
				t.Fatalf("open db error: %v", err)
			}
			defer db.Close()
			gotVersion, err := SearchPgVersionInDB(db)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchPgVersionInDB() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotVersion, tt.wantVersion) {
				t.Errorf("SearchPgVersionInDB() gotVersion = %v, want %v", gotVersion, tt.wantVersion)
			}
		})
	}
}
