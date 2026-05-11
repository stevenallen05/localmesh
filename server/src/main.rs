use tonic::{transport::Server, Request, Response, Status};

mod test {
    tonic::include_proto!("test");
}

use test::test_server::{Test, TestServer};
use test::{TestRequest, TestResponse};

#[derive(Default)]
pub struct MyTest {}

#[tonic::async_trait]
impl Test for MyTest {
    async fn test_method(
        &self,
        request: Request<TestRequest>,
    ) -> Result<Response<TestResponse>, Status> {
        let r = request.into_inner();
        println!("Got a request for: {:?}", &r);

        let reply = TestResponse {
            test_after: format!("Processed: {}", r.test_before),
        };
        Ok(Response::new(reply))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = "127.0.0.1:3000".parse().unwrap();
    let server = MyTest::default();

    println!("TestServer listening on {}", addr);

    Server::builder()
        .add_service(TestServer::new(server))
        .serve(addr)
        .await?;

    Ok(())
}




