package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rwoj/sample-micro-app/api-server/database"
	"github.com/rwoj/sample-micro-app/api-server/smicroapppb/smicroapppb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	r := gin.Default()
	cc, err := grpc.Dial("grpc-server-srv:50051", grpc.WithInsecure())
	// cc, err := grpc.Dial("grpc-server-srv", grpc.WithInsecure())
	pc := smicroapppb.NewCalculatorServiceClient(cc)
	if err != nil {
		log.Fatalf("could not connect: %v", err)
	}
	defer cc.Close()
	fmt.Println("Created grpc client")
	// fmt.Printf("Created grpc client: %f", pc)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "heja tu pong",
		})
	})
	r.GET("/animal/:name", func(c *gin.Context) {
		animal, err := database.GetAnimal(c.Param("name"))
		if err != nil {
			c.String(404, err.Error())
			return
		}
		c.JSON(200, animal)
	})
	r.GET("/calculator/sum/:val", func(c *gin.Context) {
		calc := c.Param("val")
		i, er := strconv.ParseInt(calc, 10, 32)
		j := 35
		if er != nil {
			log.Fatalf("not a number parameter: %v", er)
		}

		res, err := doSumUnary(pc, int32(i), int32(j))
		if err != nil {
			log.Fatalf("error while calling Sum RPC: %v", err)
		}

		log.Printf("Response from Sum: %v", res)

		t := "sum of " + calc + " and " + strconv.FormatInt(int64(j), 10)

		c.JSON(200, gin.H{
			t: res,
		})
	})
	r.GET("/calculator/prime/:val", func(c *gin.Context) {
		calc := c.Param("val")
		i, er := strconv.ParseInt(calc, 10, 64)
		if er != nil {
			log.Fatalf("not a number parameter: %v", er)
		}

		rr, err := doPrimaryServerStreaming(pc, i)
		if err != nil {
			log.Fatal("errror")
		}

		t := "prime"
		res := fmt.Sprintf("%v", rr)

		c.JSON(200, gin.H{
			t: res,
		})
	})

	r.Run(":3000")
}

// doServerStreaming(pc)

// doClientStreaming(pc)

// doBiDiStreaming(pc)

// doErrorUnary(pc)

func doSumUnary(c smicroapppb.CalculatorServiceClient, fn int32, sn int32) (rr int32, err error) {
	fmt.Println("Starting to do a Sum Unary RPC...")
	req := &smicroapppb.SumRequest{
		FirstNumber:  fn,
		SecondNumber: sn,
	}
	res, err := c.Sum(context.Background(), req)
	if err != nil {
		return 0, err
	}
	return res.SumResult, nil
}

func doPrimaryServerStreaming(c smicroapppb.CalculatorServiceClient, pn int64) (rr []int64, er error) {
	// strm smicroapppb.CalculatorService_PrimeNumberDecompositionServer
	fmt.Println("Starting to do a PrimeDecomposition Server Streaming RPC...")
	req := &smicroapppb.PrimeNumberDecompositionRequest{
		Number: pn,
	}
	strm, err := c.PrimeNumberDecomposition(context.Background(), req)
	if err != nil {
		log.Fatalf("error while calling PrimeDecomposition RPC: %v", err)
	}
	// if err0 != nil {
	// 	return err0
	// }

	for {
		res, err := strm.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Something happened: %v", err)
		}
		rr = append(rr, res.GetPrimeFactor())
		fmt.Println(res.GetPrimeFactor())
	}
	return rr, nil
}

func doClientStreaming(c smicroapppb.CalculatorServiceClient) {
	fmt.Println("Starting to do a ComputeAverage Client Streaming RPC...")

	stream, err := c.ComputeAverage(context.Background())
	if err != nil {
		log.Fatalf("Error while opening stream: %v", err)
	}

	numbers := []int32{3, 5, 9, 54, 23}

	for _, number := range numbers {
		fmt.Printf("Sending number: %v\n", number)
		stream.Send(&smicroapppb.ComputeAverageRequest{
			Number: number,
		})
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Error while receiving response: %v", err)
	}

	fmt.Printf("The Average is: %v\n", res.GetAverage())
}

func doBiDiStreaming(c smicroapppb.CalculatorServiceClient) {
	fmt.Println("Starting to do a FindMaximum BiDi Streaming RPC...")

	stream, err := c.FindMaximum(context.Background())

	if err != nil {
		log.Fatalf("Error while opening stream and calling FindMaximum: %v", err)
	}

	waitc := make(chan struct{})

	// send go routine
	go func() {
		numbers := []int32{4, 7, 2, 19, 4, 6, 32}
		for _, number := range numbers {
			fmt.Printf("Sending number: %v\n", number)
			stream.Send(&smicroapppb.FindMaximumRequest{
				Number: number,
			})
			time.Sleep(1000 * time.Millisecond)
		}
		stream.CloseSend()
	}()
	// receive go routine
	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("Problem while reading server stream: %v", err)
				break
			}
			maximum := res.GetMaximum()
			fmt.Printf("Received a new maximum of...: %v\n", maximum)
		}
		close(waitc)
	}()
	<-waitc
}

func doErrorUnary(c smicroapppb.CalculatorServiceClient) {
	fmt.Println("Starting to do a SquareRoot Unary RPC...")

	// correct call
	doErrorCall(c, 10)

	// error call
	doErrorCall(c, -2)
}

func doErrorCall(c smicroapppb.CalculatorServiceClient, n int32) {
	res, err := c.SquareRoot(context.Background(), &smicroapppb.SquareRootRequest{Number: n})

	if err != nil {
		respErr, ok := status.FromError(err)
		if ok {
			// actual error from gRPC (user error)
			fmt.Printf("Error message from server: %v\n", respErr.Message())
			fmt.Println(respErr.Code())
			if respErr.Code() == codes.InvalidArgument {
				fmt.Println("We probably sent a negative number!")
				return
			}
		} else {
			log.Fatalf("Big Error calling SquareRoot: %v", err)
			return
		}
	}
	fmt.Printf("Result of square root of %v: %v\n", n, res.GetNumberRoot())
}
