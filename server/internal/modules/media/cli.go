package media

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"draftcms-lite/internal/web"
)

// Make = `php artisan make:media {title?} {--type=}` → `make media {title}` (TYPE=public|private opsional).
// Title/tipe kosong → ditanya interaktif, sama seperti source.
func Make(a *web.App, title, kind string) error {
	s, in := Store{a.DB}, bufio.NewReader(os.Stdin)
	for {
		for title == "" {
			t, err := ask(in, "Container title: ")
			if err != nil {
				return err
			}
			title = t
		}
		h := Slug(title)
		if _, err := s.Container(h); err == nil {
			fmt.Printf("A container with handle %q already exists. Choose a different title.\n", h)
			title = ""
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		break
	}
	for kind != "public" && kind != "private" {
		k, err := ask(in, "Container type [public]: (public, private) ")
		if err != nil {
			return err
		}
		if kind = strings.ToLower(k); kind == "" {
			kind = "public"
		}
	}
	c, err := s.CreateContainer(title, kind)
	if err != nil {
		return err
	}
	fmt.Printf("Container %q (%s) created on the %q disk.\n", c.Title, c.Handle, c.Disk)
	return nil
}

// Remove = `php artisan remove:media {handle}`: minta konfirmasi sebelum menghapus container + semua file.
func Remove(a *web.App, handle string) error {
	s := Store{a.DB}
	c, err := s.Container(handle)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("No container found with handle %q.", handle)
	} else if err != nil {
		return err
	}
	var n int
	a.DB.QueryRow(`SELECT COUNT(*) FROM assets_meta WHERE container=?`, handle).Scan(&n)
	fmt.Printf("This will permanently delete the %q (%s) container and its %d file(s). This cannot be undone.\n", c.Title, handle, n)
	if ans, err := ask(bufio.NewReader(os.Stdin), "Are you sure you want to continue? (yes/no) [no] "); err != nil || !strings.EqualFold(ans, "yes") && !strings.EqualFold(ans, "y") {
		fmt.Println("Aborted.")
		return err
	}
	if err := s.RemoveContainer(c); err != nil {
		return err
	}
	fmt.Printf("Container %q removed.\n", handle)
	return nil
}

func ask(in *bufio.Reader, q string) (string, error) {
	fmt.Print(q)
	l, err := in.ReadString('\n')
	if err != nil && (err != io.EOF || l == "") {
		return "", errors.New("no input (pass the title as argument, e.g. make media \"Blog Images\")")
	}
	return strings.TrimSpace(l), nil
}
