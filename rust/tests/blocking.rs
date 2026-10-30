//! The synchronous client (`blocking` feature).
#![cfg(feature = "blocking")]

use agdirlabs_email::blocking::AgdirEmail;
use agdirlabs_email::{ErrorCode, ListOptions, SendEmail};
use serde_json::json;
use wiremock::matchers::{method, path, query_param};
use wiremock::{Mock, MockServer, ResponseTemplate};

/// The stub server runs on its own multi-threaded runtime; the blocking client is called
/// from the plain test thread, as in a real synchronous program.
fn server() -> (tokio::runtime::Runtime, MockServer) {
    let rt = tokio::runtime::Runtime::new().unwrap();
    let server = rt.block_on(MockServer::start());
    (rt, server)
}

#[test]
fn sends_reads_and_reports_errors() {
    let (rt, server) = server();
    rt.block_on(async {
        Mock::given(method("POST"))
            .and(path("/emails"))
            .respond_with(ResponseTemplate::new(200).set_body_json(json!({ "id": "em_1" })))
            .mount(&server)
            .await;
        Mock::given(method("GET"))
            .and(path("/emails/missing"))
            .respond_with(ResponseTemplate::new(404).set_body_json(json!({
                "statusCode": 404, "name": "not_found", "message": "Email not found"
            })))
            .mount(&server)
            .await;
    });

    let agdir = AgdirEmail::builder()
        .api_key("re_test")
        .base_url(server.uri())
        .build()
        .unwrap();
    let sent = agdir
        .emails()
        .send(SendEmail::new("a@acme.com", "b@example.com", "s").text("t"))
        .unwrap();
    assert_eq!(sent.id, "em_1");

    let err = agdir.emails().get("missing").unwrap_err();
    assert!(err.is(&ErrorCode::NotFound));

    let requests = rt.block_on(server.received_requests()).unwrap();
    assert!(requests[0].headers.get("idempotency-key").is_some());
}

#[test]
fn list_all_is_an_iterator_over_every_page() {
    let (rt, server) = server();
    let domain = |id: &str| {
        json!({
            "object": "domain", "id": id, "name": format!("{id}.example.com"), "status": "verified",
            "created_at": "2026-10-30T10:00:00Z", "region": "eu-north-1"
        })
    };
    rt.block_on(async {
        Mock::given(method("GET"))
            .and(path("/domains"))
            .and(query_param("after", "d_1"))
            .respond_with(ResponseTemplate::new(200).set_body_json(
                json!({ "object": "list", "has_more": false, "data": [domain("d_2")] }),
            ))
            .mount(&server)
            .await;
        Mock::given(method("GET"))
            .and(path("/domains"))
            .respond_with(ResponseTemplate::new(200).set_body_json(
                json!({ "object": "list", "has_more": true, "data": [domain("d_1")] }),
            ))
            .mount(&server)
            .await;
    });

    let agdir = AgdirEmail::builder()
        .api_key("re_test")
        .base_url(server.uri())
        .build()
        .unwrap();
    let names: Vec<String> = agdir
        .domains()
        .list_all(ListOptions::new().limit(1))
        .map(|d| d.unwrap().name)
        .collect();
    assert_eq!(names, ["d_1.example.com", "d_2.example.com"]);
}
