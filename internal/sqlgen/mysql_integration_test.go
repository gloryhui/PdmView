//go:build darwin || linux

package sqlgen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type mysql80Sandbox struct {
	client    string
	socket    string
	loginFile string
}

func TestMySQL80(t *testing.T) {
	sandbox := startMySQL80(t)
	document, options := goldenDatabaseFixture()
	script := GenerateDatabase(document, options)
	if script.HasErrors() || script.SQL == "" {
		t.Fatalf("golden generation: %+v", script.Diagnostics)
	}
	mysqlExpect(t, sandbox, "script execution", script.SQL, "")
	mysqlExpect(t, sandbox, "schema defaults", `
SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME
FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='pdmview_golden';`, "utf8mb4\tutf8mb4_bin")
	mysqlExpect(t, sandbox, "tables and comments", `
SELECT TABLE_NAME, ENGINE, TABLE_COMMENT
FROM information_schema.TABLES WHERE TABLE_SCHEMA='pdmview_golden'
ORDER BY LOWER(TABLE_NAME);`,
		"alpha\tInnoDB\t\nSales`Table\tInnoDB\t客户表\nzeta\tInnoDB\t")
	mysqlExpect(t, sandbox, "fields and comments", `
SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_COMMENT
FROM information_schema.COLUMNS WHERE TABLE_SCHEMA='pdmview_golden'
ORDER BY LOWER(TABLE_NAME), ORDINAL_POSITION;`,
		"alpha\tid\tint\tNO\t\n"+
			"alpha\tzeta_id\tint\tYES\t\n"+
			"alpha\tparent_id\tint\tYES\t\n"+
			"Sales`Table\ttenant\tint\tNO\t\n"+
			"Sales`Table\tid\tint\tNO\t\n"+
			"Sales`Table\tdisplay`name\tvarchar(20)\tYES\t名称 \"x\"\\line\n下一行\n"+
			"Sales`Table\tstatus\tchar(1)\tYES\t\n"+
			"Sales`Table\tcreated\tdatetime(6)\tNO\t\n"+
			"Sales`Table\tnote\ttext\tYES\t\n"+
			"Sales`Table\tpayload\tjson\tYES\t\n"+
			"Sales`Table\tamount\tdecimal(20,2)\tYES\t\n"+
			"Sales`Table\tdeleted_at\tdatetime(6)\tYES\t\n"+
			"zeta\tid\tint\tNO\t\nzeta\talpha_id\tint\tYES\t")
	mysqlExpect(t, sandbox, "index columns, methods and order", `
SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, SEQ_IN_INDEX, COLUMN_NAME, COLLATION,
       SUB_PART, INDEX_TYPE, INDEX_COMMENT
FROM information_schema.STATISTICS WHERE TABLE_SCHEMA='pdmview_golden'
ORDER BY LOWER(TABLE_NAME), LOWER(INDEX_NAME), SEQ_IN_INDEX;`,
		"alpha\tfk_alpha_self\t1\t1\tparent_id\tA\tNULL\tBTREE\t\n"+
			"alpha\tfk_alpha_zeta\t1\t1\tzeta_id\tA\tNULL\tBTREE\t\n"+
			"alpha\tPRIMARY\t0\t1\tid\tA\tNULL\tBTREE\t\n"+
			"Sales`Table\tix-name\t0\t1\tdisplay`name\tA\tNULL\tBTREE\t\n"+
			"Sales`Table\tix-name\t0\t2\ttenant\tA\tNULL\tBTREE\t\n"+
			"Sales`Table\tix_note_desc\t1\t1\tnote\tD\t16\tBTREE\tnote suffix\n"+
			"Sales`Table\tPRIMARY\t0\t1\tid\tA\tNULL\tBTREE\t\n"+
			"Sales`Table\tPRIMARY\t0\t2\ttenant\tA\tNULL\tBTREE\t\n"+
			"zeta\tfk_zeta_alpha\t1\t1\talpha_id\tA\tNULL\tBTREE\t\n"+
			"zeta\tPRIMARY\t0\t1\tid\tA\tNULL\tBTREE\t")
	mysqlExpect(t, sandbox, "key and constraint column mappings", `
SELECT k.TABLE_NAME, k.CONSTRAINT_NAME, c.CONSTRAINT_TYPE, k.ORDINAL_POSITION,
       k.COLUMN_NAME, k.REFERENCED_TABLE_NAME, k.REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE k
JOIN information_schema.TABLE_CONSTRAINTS c
ON c.CONSTRAINT_SCHEMA=k.CONSTRAINT_SCHEMA AND c.TABLE_NAME=k.TABLE_NAME AND c.CONSTRAINT_NAME=k.CONSTRAINT_NAME
WHERE k.CONSTRAINT_SCHEMA='pdmview_golden'
ORDER BY LOWER(k.TABLE_NAME), LOWER(k.CONSTRAINT_NAME), k.ORDINAL_POSITION;`,
		"alpha\tfk_alpha_self\tFOREIGN KEY\t1\tparent_id\talpha\tid\n"+
			"alpha\tfk_alpha_zeta\tFOREIGN KEY\t1\tzeta_id\tzeta\tid\n"+
			"alpha\tPRIMARY\tPRIMARY KEY\t1\tid\tNULL\tNULL\n"+
			"Sales`Table\tix-name\tUNIQUE\t1\tdisplay`name\tNULL\tNULL\n"+
			"Sales`Table\tix-name\tUNIQUE\t2\ttenant\tNULL\tNULL\n"+
			"Sales`Table\tPRIMARY\tPRIMARY KEY\t1\tid\tNULL\tNULL\n"+
			"Sales`Table\tPRIMARY\tPRIMARY KEY\t2\ttenant\tNULL\tNULL\n"+
			"zeta\tfk_zeta_alpha\tFOREIGN KEY\t1\talpha_id\talpha\tid\n"+
			"zeta\tPRIMARY\tPRIMARY KEY\t1\tid\tNULL\tNULL")
	mysqlExpect(t, sandbox, "foreign key actions", `
SELECT CONSTRAINT_NAME, UPDATE_RULE, DELETE_RULE FROM information_schema.REFERENTIAL_CONSTRAINTS
WHERE CONSTRAINT_SCHEMA='pdmview_golden' ORDER BY CONSTRAINT_NAME;`,
		"fk_alpha_self\tNO ACTION\tNO ACTION\n"+
			"fk_alpha_zeta\tNO ACTION\tNO ACTION\n"+
			"fk_zeta_alpha\tNO ACTION\tNO ACTION")
	mysqlExpect(t, sandbox, "default and identity metadata", `
SELECT SUM(COLUMN_NAME='id' AND EXTRA='auto_increment'),
       SUM(COLUMN_NAME='status' AND COLUMN_DEFAULT=''),
       SUM(COLUMN_NAME='created' AND UPPER(COLUMN_DEFAULT)='CURRENT_TIMESTAMP(6)'),
       SUM(COLUMN_NAME='note' AND EXTRA='DEFAULT_GENERATED'),
       SUM(COLUMN_NAME='payload' AND EXTRA='DEFAULT_GENERATED'),
       SUM(COLUMN_NAME='amount' AND COLUMN_DEFAULT='12.30'),
       SUM(COLUMN_NAME='deleted_at' AND COLUMN_DEFAULT IS NULL)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA='pdmview_golden' AND TABLE_NAME=CONCAT('Sales', CHAR(96), 'Table');`, "1\t1\t1\t1\t1\t1\t1")
	mysqlExpect(t, sandbox, "effective defaults, identity and escaping",
		"USE `pdmview_golden`;\n"+
			"INSERT INTO `Sales``Table` (`tenant`) VALUES (7);\n"+
			"SELECT `id`, HEX(`display``name`), `status`='', `created` IS NOT NULL, `note`, "+
			"JSON_LENGTH(`payload`), `amount`, `deleted_at` IS NULL FROM `Sales``Table`;",
		"1\t6C696E650A6E6578745C70617468\t1\t1\thello\t0\t12.30\t1")
	_, err := sandbox.run("INSERT INTO `pdmview_golden`.`alpha` (`id`, `zeta_id`) VALUES (1, 999);")
	if err == nil || !strings.Contains(err.Error(), "ERROR 1452") {
		t.Fatalf("expected MySQL missing parent error 1452, got %v", err)
	}
	t.Log("MySQL rejected missing parent with error 1452")
}

func mysqlExpect(t *testing.T, sandbox mysql80Sandbox, label, sql, expected string) {
	t.Helper()
	output, err := sandbox.run(sql)
	if err != nil || strings.TrimSuffix(output, "\n") != expected {
		t.Fatalf("%s: %v\nwant:\n%q\ngot:\n%q", label, err, expected, output)
	}
	t.Logf("%s matched", label)
}

func startMySQL80(t *testing.T) mysql80Sandbox {
	t.Helper()
	binDir := os.Getenv("PDMVIEW_MYSQL80_BIN")
	if binDir == "" {
		t.Skip("set PDMVIEW_MYSQL80_BIN to a local MySQL 8.0 bin directory")
	}
	if !filepath.IsAbs(binDir) {
		t.Fatal("PDMVIEW_MYSQL80_BIN must be absolute")
	}
	mysqld := filepath.Join(binDir, "mysqld")
	version, err := exec.Command(mysqld, "--no-defaults", "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(version), " Ver 8.0.") {
		t.Fatalf("expected a MySQL 8.0 server, got %q (%v)", version, err)
	}
	dir := t.TempDir()
	sandbox := mysql80Sandbox{
		client:    filepath.Join(binDir, "mysql"),
		socket:    filepath.Join(dir, "m.sock"),
		loginFile: filepath.Join(dir, "unused-login.cnf"),
	}
	if len(sandbox.socket) > 103 {
		t.Fatal("MySQL socket path is too long; set TMPDIR to a shorter ignored workspace directory")
	}

	dataDir := filepath.Join(dir, "data")
	initCtx, cancelInit := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancelInit()
	initialize := exec.CommandContext(initCtx, mysqld, "--no-defaults", "--initialize-insecure", "--datadir="+dataDir)
	initialize.Dir = dir
	if output, err := initialize.CombinedOutput(); err != nil {
		t.Fatalf("initialize isolated MySQL: %v\n%s", err, output)
	}

	serverLog, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverLog.Close() })
	server := exec.Command(mysqld, "--no-defaults", "--datadir="+dataDir,
		"--socket="+sandbox.socket, "--pid-file="+filepath.Join(dir, "mysqld.pid"),
		"--tmpdir="+dir, "--skip-networking", "--mysqlx=OFF", "--skip-log-bin",
		"--persisted-globals-load=OFF")
	server.Dir = dir
	server.Stdout, server.Stderr = serverLog, serverLog
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var serverErr error
	go func() {
		serverErr = server.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case <-done:
			return
		default:
		}
		_ = server.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
			if serverErr != nil {
				t.Errorf("isolated MySQL shutdown: %v", serverErr)
			}
		case <-time.After(10 * time.Second):
			_ = server.Process.Kill()
			<-done
			t.Error("isolated MySQL shutdown timed out")
		}
	})

	waitUntil := time.Now().Add(20 * time.Second)
	for time.Now().Before(waitUntil) {
		select {
		case <-done:
			output, _ := os.ReadFile(serverLog.Name())
			t.Fatalf("isolated MySQL exited on startup: %v\n%s", serverErr, output)
		default:
		}
		output, err := sandbox.run("SELECT VERSION(), @@skip_networking, @@socket, @@datadir;")
		if err == nil {
			expected := "\t1\t" + sandbox.socket + "\t" + dataDir + "/"
			if !strings.HasPrefix(output, "8.0.") || !strings.HasSuffix(output, expected+"\n") {
				t.Fatalf("unexpected MySQL fixture identity: %q", output)
			}
			version, _, _ := strings.Cut(output, "\t")
			t.Logf("MySQL %s, temporary socket only, TCP disabled", version)
			return sandbox
		}
		time.Sleep(100 * time.Millisecond)
	}
	output, _ := os.ReadFile(serverLog.Name())
	t.Fatalf("isolated MySQL did not become ready\n%s", output)
	return mysql80Sandbox{}
}

func (sandbox mysql80Sandbox) run(input string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, sandbox.client, "--no-defaults", "--protocol=SOCKET",
		"--socket="+sandbox.socket, "--user=root", "--connect-timeout=2",
		"--batch", "--skip-column-names", "--raw", "--binary-mode", "--default-character-set=utf8mb4")
	command.Env = append(command.Environ(), "MYSQL_TEST_LOGIN_FILE="+sandbox.loginFile, "MYSQL_PWD=")
	command.Stdin = strings.NewReader(input)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("MySQL client: %w", ctx.Err())
	}
	if err != nil {
		return string(output), fmt.Errorf("MySQL client: %w: %s", err, stderr.String())
	}
	return string(output), nil
}
