package managers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var ErrTokenNotFound = errors.New("token not found")
var ErrNotFound = errors.New("item not found")
var ErrInternal = errors.New("internal error")
var ErrNoSuchUser = errors.New("no such user")
var ErrInvalidPassword = errors.New("invalid password")
var ErrPhoneUsed = errors.New("phone alredy registered")
var ErrTokenExpired = errors.New("token expired")

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

type Manager struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Salary      int64     `json:"salary"`
	Plan        int64     `json:"plan"`
	BossID      int64     `json:"boss_id"`
	Departament string    `json:"departament"`
	Phone       string    `json:"phone"`
	Password    string    `json:"password"`
	IsAdmin     bool      `json:"is_admin"`
	Created     time.Time `json:"created"`
}

type Product struct {
	ID      int64     `json:"id"`
	Name    string    `json:"name"`
	Price   int       `json:"price"`
	Qty     int       `json:"qty"`
	Active  bool      `json:"active"`
	Created time.Time `json:"created"`
}

type Sale struct {
	ID         int64           `json:"id"`
	ManagerID  int64           `json:"manager_id"`
	CustomerID int64           `json:"customer_id"`
	Created    time.Time       `json:"created"`
	Positions  []*SalePosition `json:"positions"`
}

type SalePosition struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	SaleID    int64     `json:"sale_id"`
	Price     int       `json:"price"`
	Qty       int       `json:"qty"`
	Created   time.Time `json:"created"`
}

type Customer struct {
	ID      int64     `json:"id"`
	Name    string    `json:"name"`
	Phone   string    `json:"phone"`
	Active  bool      `json:"active"`
	Created time.Time `json:"created"`
}

func (s *Service) IDByToken(ctx context.Context, token string) (int64, error) {
	var id int64
	sqlStatement := `select manager_id from managers_tokens where token = $1`
	err := s.db.QueryRow(ctx, sqlStatement, token).Scan(&id)
	if err != nil {
		log.Print(err)
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, nil
	}
	return id, nil
}

func (s *Service) IsAdmin(ctx context.Context, id int64) (isAdmin bool) {
	sqlStmt := `select is_admin from managers  where id = $1`
	err := s.db.QueryRow(ctx, sqlStmt, id).Scan(&isAdmin)
	if err != nil {
		return false
	}
	return
}

func (s *Service) Create(ctx context.Context, item *Manager) (string, error) {
	var token string
	var id int64

	sqlStmt := `insert into managers(name,phone,is_admin) values ($1,$2,$3) on conflict (phone) do nothing returning id;`
	err := s.db.QueryRow(ctx, sqlStmt, item.Name, item.Phone, item.IsAdmin).Scan(&id)
	if err != nil {
		log.Print(err)
		return "", ErrInternal
	}

	buffer := make([]byte, 256)
	n, err := rand.Read(buffer)
	if n != len(buffer) || err != nil {
		return "", ErrInternal
	}

	token = hex.EncodeToString(buffer)
	_, err = s.db.Exec(ctx, `insert into managers_tokens(token,manager_id) values($1,$2)`, token, id)
	if err != nil {
		return "", ErrInternal
	}
	return token, nil
}

func (s *Service) Token(ctx context.Context, phone, password string) (token string, err error) {
	var hash string
	var id int64
	err = s.db.QueryRow(ctx, `select id,password from managers where phone = $1`, phone).Scan(&id, &hash)

	if err == pgx.ErrNoRows {
		return "", ErrInvalidPassword
	}
	if err != nil {
		return "", ErrInternal
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return "", ErrInvalidPassword
	}

	buffer := make([]byte, 256)
	n, err := rand.Read(buffer)
	if n != len(buffer) || err != nil {
		return "", ErrInternal
	}

	token = hex.EncodeToString(buffer)

	_, err = s.db.Exec(ctx, `insert into managers_tokens(token,manager_id) values($1,$2)`, token, id)
	if err != nil {
		return "", ErrInternal
	}

	return token, nil
}

func (s *Service) SaveProduct(ctx context.Context, product *Product) (*Product, error) {
	var err error
	if product.ID == 0 {
		sqlstmt := `insert into products(name,qty,price) values ($1,$2,$3) returning id,name,qty,price,active,created;`
		err = s.db.QueryRow(ctx, sqlstmt, product.Name, product.Qty, product.Price).
			Scan(&product.ID, &product.Name, &product.Qty, &product.Price, &product.Active, &product.Created)
	} else {
		sqlstmt := `update  products set  name=$1, qty=$2,price=$3  where id = $4 returning id,name,qty,price,active,created;`
		err = s.db.QueryRow(ctx, sqlstmt, product.Name, product.Qty, product.Price, product.ID).
			Scan(&product.ID, &product.Name, &product.Qty, &product.Price, &product.Active, &product.Created)
	}

	if err != nil {
		log.Print(err)
		return nil, ErrInternal
	}
	return product, nil
}

func (s *Service) MakeSalePosition(ctx context.Context, position *SalePosition) bool {
	active := false
	qty := 0
	if err := s.db.QueryRow(ctx, `select qty,active from products where id = $1`, position.ProductID).
		Scan(&qty, &active); err != nil {
		return false
	}
	if qty < position.Qty || !active {
		return false
	}
	if _, err := s.db.Exec(ctx, `update products set qty = $1 where id = $2`, qty-position.Qty, position.ProductID); err != nil {
		log.Print(err)
		return false
	}
	return true
}

func (s *Service) MakeSale(ctx context.Context, sale *Sale) (*Sale, error) {

	positionsSQLstmt := "insert into sales_positions (sale_id,product_id,qty,price) values "

	sqlstmt := `insert into sales(manager_id,customer_id) values ($1,$2) returning id, created;`

	err := s.db.QueryRow(ctx, sqlstmt, sale.ManagerID, sale.CustomerID).Scan(&sale.ID, &sale.Created)
	if err != nil {
		log.Print(err)
		return nil, ErrInternal
	}
	for _, position := range sale.Positions {
		if !s.MakeSalePosition(ctx, position) {
			log.Print("Invalid position")
			return nil, ErrInternal
		}
		positionsSQLstmt += "(" + strconv.FormatInt(sale.ID, 10) + "," + strconv.FormatInt(position.ProductID, 10) + "," + strconv.Itoa(position.Price) + "," + strconv.Itoa(position.Qty) + "),"
	}

	positionsSQLstmt = positionsSQLstmt[0 : len(positionsSQLstmt)-1]

	log.Print(positionsSQLstmt)
	_, err = s.db.Exec(ctx, positionsSQLstmt)
	if err != nil {
		log.Print(err)
		return nil, ErrInternal
	}

	return sale, nil
}

func (s *Service) GetSales(ctx context.Context, id int64) (sum int, err error) {

	sqlstmt := `
	select coalesce(sum(sp.qty * sp.price),0) total
	from managers m
	left join sales s on s.manager_id= $1
	left join sales_positions sp on sp.sale_id = s.id
	group by m.id
	limit 1`

	err = s.db.QueryRow(ctx, sqlstmt, id).Scan(&sum)
	if err != nil {
		log.Print(err)
		return 0, ErrInternal
	}
	return sum, nil
}

func (s *Service) Products(ctx context.Context) ([]*Product, error) {

	items := make([]*Product, 0)

	sqlstmt := `select id, name, price, qty from products where active = true order by id limit 500`
	rows, err := s.db.Query(ctx, sqlstmt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return items, nil
		}
		return nil, ErrInternal
	}
	defer rows.Close()

	for rows.Next() {
		item := &Product{}
		err = rows.Scan(&item.ID, &item.Name, &item.Price, &item.Qty)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *Service) RemoveProductByID(ctx context.Context, id int64) (err error) {

	_, err = s.db.Exec(ctx, `delete from products where id = $1`, id)
	if err != nil {
		log.Print(err)
		return ErrInternal
	}
	return nil
}

func (s *Service) RemoveCustomerByID(ctx context.Context, id int64) (err error) {

	_, err = s.db.Exec(ctx, `DELETE from customers where id = $1`, id)
	if err != nil {
		log.Print(err)
		return ErrInternal
	}
	return nil
}

func (s *Service) Customers(ctx context.Context) ([]*Customer, error) {

	items := make([]*Customer, 0)
	sqlstmt := `select id, name, phone, active, created from customers where active = true order by id limit 500`
	rows, err := s.db.Query(ctx, sqlstmt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return items, nil
		}
		return nil, ErrInternal
	}
	defer rows.Close()

	for rows.Next() {
		item := &Customer{}
		err = rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Active, &item.Created)
		if err != nil {
			log.Print(err)
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func (s *Service) ChangeCustomer(ctx context.Context, customer *Customer) (*Customer, error) {

	sqlstmt := `update customers set name = $2, phone = $3, active = $4  where id = $1 returning name,phone,active`

	if err := s.db.QueryRow(ctx, sqlstmt, customer.ID, customer.Name, customer.Phone, customer.Active).
		Scan(&customer.Name, &customer.Phone, &customer.Active); err != nil {
		log.Print(err)
		return nil, ErrInternal
	}

	return customer, nil
}
