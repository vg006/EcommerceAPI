package order

import (
	"context"
	"fmt"
	"github.com/deckarep/golang-set/v2"
	"github.com/rasadov/EcommerceAPI/account"
	"github.com/rasadov/EcommerceAPI/order/pb"
	"github.com/rasadov/EcommerceAPI/product"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"strconv"
)

type grpcServer struct {
	pb.UnimplementedOrderServiceServer
	service       Service
	accountClient *account.Client
	productClient *product.Client
}

func ListenGRPC(service Service, accountURL string, productURL string, port int) error {
	accountClient, err := account.NewClient(accountURL)
	if err != nil {
		return err
	}

	productClient, err := product.NewClient(productURL)
	if err != nil {
		accountClient.Close()
		return err
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		accountClient.Close()
		productClient.Close()
		return err
	}

	serv := grpc.NewServer()
	pb.RegisterOrderServiceServer(serv, &grpcServer{
		pb.UnimplementedOrderServiceServer{},
		service,
		accountClient,
		productClient,
	})
	reflection.Register(serv)

	return serv.Serve(lis)
}

func (server *grpcServer) PostOrder(ctx context.Context, request *pb.PostOrderRequest) (*pb.PostOrderResponse, error) {
	_, err := server.accountClient.GetAccount(ctx, request.AccountId)
	if err != nil {
		log.Println("Error getting account", err)
		return nil, err
	}
	var productIDs []string
	for _, p := range request.Products {
		productIDs = append(productIDs, p.Id)
	}
	orderedProducts, err := server.productClient.GetProducts(ctx, 0, 0, productIDs, "")
	if err != nil {
		log.Println("Error getting ordered products", err)
		return nil, err
	}

	var products []OrderedProduct

	for _, p := range orderedProducts {
		productObj := OrderedProduct{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			Quantity:    0,
		}
		for _, requestProduct := range request.Products {
			if requestProduct.Id == p.ID {
				productObj.Quantity = requestProduct.Quantity
				break
			}
		}

		if productObj.Quantity != 0 {
			products = append(products, productObj)
		}
	}

	order, err := server.service.PostOrder(ctx, request.AccountId, request.GetTotalPrice(), products)
	if err != nil {
		log.Println("Error posting order", err)
		return nil, err
	}

	orderProto := &pb.Order{
		Id:         strconv.Itoa(int(order.ID)),
		AccountId:  order.AccountID,
		TotalPrice: order.TotalPrice,
		Products:   []*pb.ProductInfo{},
	}
	orderProto.CreatedAt, _ = order.CreatedAt.MarshalBinary()
	for _, p := range order.Products {
		orderProto.Products = append(orderProto.Products, &pb.ProductInfo{
			Id:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Price:       p.Price,
			Quantity:    p.Quantity,
		})
	}
	return &pb.PostOrderResponse{
		Order: orderProto,
	}, nil
}

func (server *grpcServer) GetOrdersForAccount(ctx context.Context, request *pb.GetOrdersForAccountRequest) (*pb.GetOrdersForAccountResponse, error) {
	accountOrders, err := server.service.GetOrdersForAccount(ctx, request.AccountId)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Taking unique products. We use set to avoid repeating
	productIDsSet := mapset.NewSet[string]()
	for _, o := range accountOrders {
		for _, p := range o.Products {
			productIDsSet.Add(p.ID)
		}
	}

	productIDs := productIDsSet.ToSlice()

	products, err := server.productClient.GetProducts(ctx, 0, 0, productIDs, "")
	if err != nil {
		log.Println("Error getting account products: ", err)
		return nil, err
	}

	// Collecting orders

	var orders []*pb.Order
	for _, order := range accountOrders {
		// Encode order
		encodedOrder := &pb.Order{
			AccountId:  order.AccountID,
			Id:         strconv.Itoa(int(order.ID)),
			TotalPrice: order.TotalPrice,
			Products:   []*pb.ProductInfo{},
		}
		encodedOrder.CreatedAt, _ = order.CreatedAt.MarshalBinary()

		// Decorate orders with products
		for _, orderedProduct := range order.Products {
			// Populate product fields
			for _, prod := range products {
				if prod.ID == orderedProduct.ID {
					orderedProduct.Name = prod.Name
					orderedProduct.Description = prod.Description
					orderedProduct.Price = prod.Price
					break
				}
			}

			encodedOrder.Products = append(encodedOrder.Products, &pb.ProductInfo{
				Id:          orderedProduct.ID,
				Name:        orderedProduct.Name,
				Description: orderedProduct.Description,
				Price:       orderedProduct.Price,
				Quantity:    orderedProduct.Quantity,
			})
		}

		orders = append(orders, encodedOrder)
	}
	return &pb.GetOrdersForAccountResponse{Orders: orders}, nil
}
