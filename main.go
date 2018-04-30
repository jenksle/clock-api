package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo"
)

//Config struct
type Config struct {
	ClockServer string `json:"clockServer"`
	ClockUser   string `json:"clockUser"`
	ClockPwd    string `json:"clockPwd"`
	ClockDb     string `json:"clockDb"`
	Port        int    `json:"port"`
}

type department struct {
	DepartmentID   int    `json:"departmentID"`
	DepartmentName string `json:"departmentName"`
}

type employee struct {
	FirstName  string `json:"firstName"`
	Surname    string `json:"surname"`
	EmployeeID int    `json:"employeeID"`
}

type employeeclock struct {
	EmployeeID  int    `json:"employeeID"`
	TimeID      int    `json:"timeID"`
	StartDT     string `json:"startDate"`
	StartTime   string `json:"startTime"`
	FinishDT    string `json:"finishDate"`
	TimeDiff    string `json:"timeDifference"`
	ClockInTime string `json:"clockInTime"`
}

type clockpage struct {
	FirstName      string `json:"firstName"`
	Surname        string `json:"surname"`
	EmployeeID     int    `json:"employeeID"`
	EmployeeClocks []employeeclock
	ClockDetail    string `json:"clockDetail"`
	InOut          string `json:"inOut"`
	ClockedIn      string `json:"clockedIn"`
	DepartmentName string `json:"departmentName"`
	DepartmentID   int    `json:"departmentID"`
	ServerTime     string `json:"serverTime"`
}

type clockinout struct {
	TimeID   int    `json:"timeID"`
	StartDT  string `json:"startDate"`
	FinishDT string `json:"finishDate"`
}

var db *sql.DB
var config Config

func init() {

	// Load application configuration from settings file
	file, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to the mysql database and test connection
	connection := fmt.Sprintf("%s:%s@/%s",
		config.ClockUser,
		config.ClockPwd,
		config.ClockDb)

	db, err = sql.Open("mysql", connection)
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
}

func main() {

	e := echo.New()

	e.GET("/departments", getDepartments)
	e.GET("/employees/:id", departmentEmployees)
	e.GET("/employeedetails/:id", employeeDetails)
	e.POST("/startstop/:id", startStop)

	e.Start(":8080")

}

func getDepartments(c echo.Context) error {

	sqlGetDepartment := `select department_id, department_name from tbl_department`

	rows, err := db.Query(sqlGetDepartment)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var d department
	departments := make([]department, 0)

	for rows.Next() {

		err := rows.Scan(&d.DepartmentID, &d.DepartmentName)
		if err != nil {
			log.Fatal(err)
		}
		departments = append(departments, d)

	}

	return c.JSON(http.StatusOK, departments)

}

func departmentEmployees(c echo.Context) error {

	departmentID := c.Param("id")

	sqlDepartmentEmployees := `SELECT first_name, surname, employee_id
							   FROM tbl_employee e
							   WHERE is_active = 'Y'
	  								AND department_id = ?
							   ORDER BY surname ASC`

	rows, err := db.Query(sqlDepartmentEmployees, departmentID)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	var e employee
	employees := make([]employee, 0)

	for rows.Next() {

		err := rows.Scan(&e.FirstName, &e.Surname, &e.EmployeeID)
		if err != nil {
			log.Fatal(err)
		}

		employees = append(employees, e)

	}

	return c.JSON(http.StatusOK, employees)

}

func employeeDetails(c echo.Context) error {

	employeeID := c.Param("id")

	var output clockpage

	sqldept := `SELECT d.department_name, d.department_id 
				FROM tbl_employee e
					INNER JOIN tbl_department d ON d.department_id = e.department_id
				WHERE employee_id = ?`

	err := db.QueryRow(sqldept, employeeID).Scan(&output.DepartmentName, &output.DepartmentID)
	if err != nil {
		log.Fatal(err)
	}

	var e employeeclock

	sql3 := `SELECT first_name, surname, employee_id
				FROM tbl_employee
				WHERE employee_id = ?`

	err = db.QueryRow(sql3, employeeID).Scan(&output.FirstName, &output.Surname, &output.EmployeeID)
	if err != nil {
		log.Fatal(err)
	}

	sql4 := `SELECT e.employee_id, t.time_id, DATE_FORMAT(t.start_dt, '%W, %D %M'), DATE_FORMAT(t.start_dt, '%T'), COALESCE(DATE_FORMAT(t.finish_dt, '%T'), '-'), COALESCE(TIMEDIFF(t.finish_dt, t.start_dt), '-')
				FROM tbl_employee e
					INNER JOIN tbl_time t ON t.employee_id=e.employee_id
				WHERE e.employee_id = ?
				ORDER BY t.time_id DESC
				LIMIT 5`

	rows, err := db.Query(sql4, employeeID)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	output.EmployeeClocks = make([]employeeclock, 0)

	for rows.Next() {

		err := rows.Scan(&e.EmployeeID, &e.TimeID, &e.StartDT, &e.StartTime, &e.FinishDT, &e.TimeDiff)
		if err != nil {
			log.Fatal(err)
		}

		output.EmployeeClocks = append(output.EmployeeClocks, e)
	}

	sql5 := `SELECT e.employee_id, t.time_id, DATE_FORMAT(t.start_dt, '%W, %D %M'), COALESCE(DATE_FORMAT(t.finish_dt, '%Y-%m-%d %T'), '-'), TIMEDIFF(NOW(), t.start_dt)
				FROM tbl_employee e
					INNER JOIN tbl_time t ON t.employee_id=e.employee_id
				WHERE e.employee_id = ?
				ORDER BY t.time_id DESC
				LIMIT 1`

	err = db.QueryRow(sql5, employeeID).Scan(&e.EmployeeID, &e.TimeID, &e.StartDT, &e.FinishDT, &e.ClockInTime)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	if e.FinishDT == "-" {
		output.ClockDetail = "clocked in"
		output.InOut = "Clock out"
		output.ClockedIn = e.ClockInTime

	} else {
		output.ClockDetail = "clocked out"
		output.InOut = "Clock in"
		output.ClockedIn = "00:00"
	}

	output.ServerTime = time.Now().Format("15 : 04 : 05")

	return c.JSON(http.StatusOK, output)

}

func startStop(c echo.Context) error {

	employeeID := c.Param("id")

	var e clockinout

	currentTime := time.Now().Format("02 Jan 06 15:04 MST")

	sql6 := `SELECT t.time_id, DATE_FORMAT(t.start_dt, '%Y-%m-%d %T'), COALESCE(DATE_FORMAT(t.finish_dt, '%Y-%m-%d %T'), '-')
			 FROM tbl_employee e
				 INNER JOIN tbl_time t ON t.employee_id=e.employee_id
			 WHERE e.employee_id = ?
			 ORDER BY t.time_id DESC
			 LIMIT 1`

	err := db.QueryRow(sql6, employeeID).Scan(&e.TimeID, &e.StartDT, &e.FinishDT)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal(err)
	}

	if e.FinishDT == "-" {

		sql7 := `UPDATE tbl_time
				 SET finish_dt = NOW()
					WHERE time_id = ?;`

		_, err := db.Exec(sql7, e.TimeID)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Clocked off at " + currentTime)

	} else {

		sql8 := `INSERT INTO tbl_time (employee_id, start_dt)
				 VALUES (?, NOW())`

		_, err := db.Exec(sql8, employeeID)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Clocked in at " + currentTime)

	}
	return c.JSON(http.StatusOK, e)
}
