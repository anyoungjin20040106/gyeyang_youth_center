package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3" // SQLite 드라이버 임포트
)

type PageData struct {
	Table template.HTML
}

func generateSecretHex(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Fatalf("비밀 키 생성 실패: %v", err)
	}
	return hex.EncodeToString(bytes)
}

func main() {
	// Gin 라우터 생성
	r := gin.Default()

	// SQLite 데이터베이스 연결
	db, err := sql.Open("sqlite3", "./main.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 세션 스토어 설정
	store := cookie.NewStore([]byte(generateSecretHex(32)))
	r.Use(sessions.Sessions("session", store))

	// HTML 템플릿 파일 로드
	r.LoadHTMLGlob("templates/*")

	// 메인 페이지 렌더링
	r.Any("/", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		if session.Get("user") != nil {
			ctx.Redirect(303, "/ppt")

		} else {
			msg := session.Get("msg")
			color := session.Get("color")

			// msg와 color가 있는 경우 템플릿에 전달
			data := gin.H{}
			if msg != nil {
				data["msg"] = msg
				data["color"] = color

				// 세션에서 메시지 정보를 삭제하여 한 번만 표시
				session.Delete("msg")
				session.Delete("color")
				session.Delete("failMsg")
				session.Save()
			}

			ctx.HTML(http.StatusOK, "index.html", data)
		}
	})

	// 로그인 처리 핸들러
	r.POST("/login", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		pw := ctx.PostForm("pw")

		// 비밀번호 검증 및 세션 설정
		if pw == os.Getenv("Ttest") {
			session.Set("user", "T")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/ppt")
		} else if pw == os.Getenv("Stest") {
			session.Set("user", "S")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/ppt")
		} else {
			// 비밀번호가 틀린 경우 세션에 오류 메시지 저장하고 리다이렉트
			session.Set("msg", "잘못된 비밀번호입니다.")
			session.Set("color", "red")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/")
		}
	})

	// PPT 페이지 접근 핸들러
	r.Any("/ppt", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		user := session.Get("user")
		// 사용자 로그인 여부 확인
		if user == "T" || user == "S" {
			rows, err := db.Query("SELECT * FROM PPT")
			if err != nil {
				session.Set("msg", "DB 오류.")
				session.Set("color", "red")
				session.Save()
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}
			defer rows.Close()
			table := "<tr><th>제목</th><th>업로드 날짜</th><th>업로드 시간</th><th>다운로드</th>"
			if user == "T" {
				table += "<th>삭제하기</th>"
			}
			table += "</tr>"
			for rows.Next() {
				var title string
				var upload string
				var fname string
				rows.Scan(&title, &upload, &fname)
				tdata := strings.Split(strings.Replace(strings.Replace(upload, "Z", "", 1), "T", " ", 1), " ")
				table += fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td><a href='download?fname=%s'>다운로드</a></td>", title, tdata[0], tdata[1], fname)
				if user == "T" {
					table += fmt.Sprintf("<td><a href='delete?fname=%s'>삭제하기</a></td>", fname)
				}
				table += "<tr>"
			}
			ctx.HTML(http.StatusOK, "ppt.html", gin.H{"user": user, "table": template.HTML(table)})
		} else {
			// 로그인하지 않은 경우 메인 페이지로 리다이렉트
			session.Set("msg", "암호를 입력해주세요.")
			session.Set("color", "red")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/")
		}
	})
	r.Any("/getDB", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		user := session.Get("user")

		// 사용자 로그인 여부 확인
		if user == "T" {
			ctx.FileAttachment("main.db", "main.db")
			files, _ := db.Query("select fname from PPT")
			for files.Next() {
				var file string
				files.Scan(&file)
				ctx.FileAttachment(filepath.Join("ppt", file), file)
			}
			ctx.Redirect(http.StatusSeeOther, "/ppt")
		} else {
			session.Set("msg", "선생님만 들어갈수 있습니다.")
			session.Set("color", "red")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/")
		}

	})
	r.Any("/download", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		user := session.Get("user")
		fname := ctx.Query("fname")

		// 사용자 로그인 여부 확인
		if user == "T" || user == "S" {
			ctx.FileAttachment(filepath.Join("ppt", fname), fname)
			ctx.Redirect(http.StatusSeeOther, "/ppt")
		} else {
			session.Set("msg", "암호를 입력해주세요.")
			session.Set("color", "red")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/")
		}
	})
	r.Any("/delete", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		user := session.Get("user")
		// 사용자 로그인 여부 확인
		if user == "T" || user == "S" {
			fname := ctx.Query("fname")
			os.Remove(filepath.Join("ppt", fname))
			_, err := db.Exec("delete from ppt where fname=?", fname)
			if err != nil {
				fmt.Print("에러가 생겼다")
			}
			ctx.Redirect(http.StatusSeeOther, "/ppt")
		} else {
			session.Set("msg", "암호를 입력해주세요.")
			session.Set("color", "red")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/")
		}
	})
	r.Any("/teacher", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		user := session.Get("user")

		// 사용자 로그인 여부 확인
		if user == "T" {
			data := gin.H{}
			session := sessions.Default(ctx)
			failMsg := session.Get("failMsg")
			if failMsg != nil {
				data["failMsg"] = failMsg
				session.Delete("failMsg")
				session.Save()
			}

			ctx.HTML(http.StatusOK, "upload.html", data)
		} else {
			session.Set("msg", "선생님만 들어갈수 있습니다.")
			session.Set("color", "red")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/")
		}
	})
	r.POST("/upload", func(ctx *gin.Context) {
		session := sessions.Default(ctx)

		// 제목 가져오기
		title := ctx.PostForm("title")

		// 파일 가져오기
		file, err := ctx.FormFile("file")
		if err != nil {
			// 파일 업로드 실패 처리
			session.Set("failMsg", "업로드 실패 : 파일 에러")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/teacher")
			return
		}

		// 파일 저장 경로 및 파일 저장
		filePath := filepath.Join("./ppt", file.Filename)
		if err := ctx.SaveUploadedFile(file, filePath); err != nil {
			// 파일 저장 실패 처리
			session.Set("failMsg", "업로드 실패 : 파일 에러")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/teacher")
			return
		}

		// 데이터베이스에 파일 제목과 경로 저장
		if _, err = db.Exec("INSERT INTO PPT (title, fname) VALUES (?, ?)", title, file.Filename); err != nil {
			// 데이터베이스 저장 실패 처리
			session.Set("failMsg", "업로드 실패 : DB 에러")
			session.Save()
			ctx.Redirect(http.StatusSeeOther, "/teacher")
			return
		}
		session.Set("failMsg", "업로드 성공")
		session.Save()
		ctx.Redirect(http.StatusSeeOther, "/ppt")
	})

	// 메인 페이지 렌더링
	r.GET("/logout", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		session.Clear()
		session.Set("failMsg", "업로드 실패 : 파일 에러")
		session.Save()
		ctx.Redirect(303, "/")
	})

	r.Run(":4321")
}
