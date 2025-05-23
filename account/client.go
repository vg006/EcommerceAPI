package account

import (
	"context"
	"github.com/rasadov/EcommerceAPI/account/pb"
	"google.golang.org/grpc"
	"strconv"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.AccountServiceClient
}

func NewClient(url string) (*Client, error) {
	conn, err := grpc.Dial(url, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	C := pb.NewAccountServiceClient(conn)
	return &Client{conn, C}, nil
}

func (client *Client) Close() {
	client.conn.Close()
}

func (client *Client) Register(ctx context.Context, name, email, password string) (string, error) {
	response, err := client.service.RegisterAccount(ctx, &pb.RegisterRequest{
		Name:     name,
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", err
	}
	return response.Token, nil
}

func (client *Client) Login(ctx context.Context, email, password string) (string, error) {
	response, err := client.service.LoginAccount(ctx, &pb.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", err
	}
	return response.Token, nil
}

func (client *Client) GetAccount(ctx context.Context, Id string) (*Account, error) {
	r, err := client.service.GetAccount(
		ctx,
		&pb.GetAccountRequest{Id: Id},
	)
	if err != nil {
		return nil, err
	}
	accountId, _ := strconv.ParseInt(r.Account.GetId(), 10, 64)
	return &Account{
		ID:    uint(accountId),
		Name:  r.Account.GetName(),
		Email: r.Account.GetEmail(),
	}, nil
}

func (client *Client) GetAccounts(ctx context.Context, skip, take uint64) ([]Account, error) {
	r, err := client.service.GetAccounts(
		ctx,
		&pb.GetAccountsRequest{Take: take, Skip: skip},
	)
	if err != nil {
		return nil, err
	}
	var accounts []Account
	for _, a := range r.Accounts {
		accountId, _ := strconv.ParseInt(a.GetId(), 10, 64)
		accounts = append(accounts, Account{
			ID:    uint(accountId),
			Name:  a.GetName(),
			Email: a.GetEmail(),
		})
	}
	return accounts, nil
}
