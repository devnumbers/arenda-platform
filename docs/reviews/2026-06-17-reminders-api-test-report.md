# Reminders API E2E Test Report

- **Date:** 2026-06-18T09:14:55Z
- **Commit:** f02aab0ff123fe604960271a187b01ae732c6f98
- **Base URL:** http://localhost:8080
- **Phone:** +79150380663

## Bruno CLI Results

Results saved to: /Users/smirnowwwivan/Nambers/arenda-planform/tools/bruno/reminders-e2e-results.json

```json
[
  {
    "iterationIndex": 0,
    "results": [
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create property main.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"name\": \"E2E Reminders Main 1781774086057\",\n  \"type\": \"apartment\",\n  \"address\": \"Main St 1781774086057\",\n  \"description\": \"E2E reminders test property main\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "09f274b8-8ed1-4a0d-a643-07c9277723d7",
            "date": "Thu, 18 Jun 2026 09:14:46 GMT",
            "content-length": "323"
          },
          "data": {
            "address": "Main St 1781774086057",
            "created_at": "2026-06-18T12:14:46.089985+03:00",
            "description": "E2E reminders test property main",
            "id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "name": "E2E Reminders Main 1781774086057",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:46.089985+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 26,
          "duration": 26,
          "size": 323
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "KYQqQ8FSXWwg8CbIL4iI0"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "zfM-2-b_ybLtzRiJB8ySU"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.240516125,
        "name": "Create Property Main",
        "path": "reminders-e2e/00-setup/create property main",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create tenant contact.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/tenant-contacts",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"name\": \"Ivan\",\n  \"surname\": \"Ivanov\",\n  \"patronymic\": \"Ivanovich\",\n  \"phone\": \"+79154086271\",\n  \"email\": \"tenant-1781774086271@example.com\",\n  \"comment\": \"Primary tenant contact\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "c82c3d9f-f3ca-4515-b44d-80e3cdece058",
            "date": "Thu, 18 Jun 2026 09:14:46 GMT",
            "content-length": "351"
          },
          "data": {
            "comment": "Primary tenant contact",
            "created_at": "2026-06-18T12:14:46.275136+03:00",
            "email": "tenant-1781774086271@example.com",
            "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
            "name": "Ivan",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "patronymic": "Ivanovich",
            "phone": "+79154086271",
            "surname": "Ivanov",
            "updated_at": "2026-06-18T12:14:46.275136+03:00"
          },
          "url": "http://localhost/tenant-contacts",
          "responseTime": 4,
          "duration": 4,
          "size": 351
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "jQFMhCCq94bqJnLjC_bqJ"
          },
          {
            "description": "should have tenant contact id",
            "status": "pass",
            "uid": "Q_sshrCVXfXq0FJSv7_oE"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.160209458,
        "name": "Create Tenant Contact",
        "path": "reminders-e2e/00-setup/create tenant contact",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create operation main.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"income\",\n  \"category\": \"rent\",\n  \"amount_kopecks\": 150000,\n  \"operation_date\": \"2026-06-25\",\n  \"comment\": \"E2E manual operation main\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "000855ad-48bb-4785-90ea-27c88b5f6e32",
            "date": "Thu, 18 Jun 2026 09:14:46 GMT",
            "content-length": "437"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E manual operation main",
            "created_at": "2026-06-18T12:14:46.434114+03:00",
            "id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-25",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T12:14:46.434114+03:00"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations",
          "responseTime": 11,
          "duration": 11,
          "size": 437
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "i64ipDAka80zW9ox_qQWJ"
          },
          {
            "description": "should have operation id",
            "status": "pass",
            "uid": "q5__f9S8tM9Ce7b0UejFz"
          },
          {
            "description": "should match operation date",
            "status": "pass",
            "uid": "5wIJJfnH4JovsNv7M8urc"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.159148209,
        "name": "Create Operation Main",
        "path": "reminders-e2e/00-setup/create operation main",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create recurring operation main.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"expense\",\n  \"category\": \"utilities\",\n  \"amount_kopecks\": 50000,\n  \"start_date\": \"2026-06-21\",\n  \"payment_day\": 21,\n  \"comment\": \"E2E recurring operation main\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "a9e6cfde-fb85-40a8-8f52-d9813364ee26",
            "date": "Thu, 18 Jun 2026 09:14:46 GMT",
            "content-length": "466"
          },
          "data": {
            "amount_kopecks": 50000,
            "category": "utilities",
            "comment": "E2E recurring operation main",
            "created_at": "2026-06-18T12:14:46.590176+03:00",
            "end_date": null,
            "id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
            "lease_id": null,
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 21,
            "periodicity": "monthly",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "start_date": "2026-06-21",
            "status": "active",
            "type": "expense",
            "updated_at": "2026-06-18T12:14:46.590176+03:00"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations",
          "responseTime": 18,
          "duration": 18,
          "size": 466
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "qIVAtgt4jQ9r2b4iW697u"
          },
          {
            "description": "should have recurring operation id",
            "status": "pass",
            "uid": "ksZld6LrxLv2Sj_b0FJA6"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.164028084,
        "name": "Create Recurring Operation Main",
        "path": "reminders-e2e/00-setup/create recurring operation main",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create property lease 30d.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"name\": \"E2E Lease 30d 1781774086749\",\n  \"type\": \"apartment\",\n  \"address\": \"Lease 30d St 1781774086749\",\n  \"description\": \"E2E reminders lease 30d property\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "c4370b46-01f5-4c2c-b246-658ccd71ec96",
            "date": "Thu, 18 Jun 2026 09:14:46 GMT",
            "content-length": "323"
          },
          "data": {
            "address": "Lease 30d St 1781774086749",
            "created_at": "2026-06-18T12:14:46.753056+03:00",
            "description": "E2E reminders lease 30d property",
            "id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
            "name": "E2E Lease 30d 1781774086749",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:46.753056+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 7,
          "duration": 7,
          "size": 323
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "kOrO1FS6Hay6nT0coNqWj"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "SA15k9UDci9nQJUQJ1SHw"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.153906209,
        "name": "Create Property Lease 30d",
        "path": "reminders-e2e/00-setup/create property lease 30d",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create lease 30d.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"property_id\": \"7cbef75c-a65c-46dd-b655-5b135c26a322\",\n  \"tenant_contact_id\": \"2cdbdd17-cef9-46f9-9134-e43d0ecee37e\",\n  \"start_date\": \"2026-06-18\",\n  \"end_date\": \"2026-07-18\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease 30d\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "4da04a8a-248e-4207-afb3-5a25f674dcb9",
            "date": "Thu, 18 Jun 2026 09:14:46 GMT",
            "content-length": "786"
          },
          "data": {
            "comment": "E2E lease 30d",
            "created_at": "2026-06-18T12:14:46.910383+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-18",
            "id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:46.910383+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 20,
          "duration": 20,
          "size": 786
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "skMMqn2HaH5Hf1Ox-8-Mb"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "MGfljt-dfpcwh_l06YLJ1"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.164578625,
        "name": "Create Lease 30d",
        "path": "reminders-e2e/00-setup/create lease 30d",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create property lease short.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"name\": \"E2E Lease Short 1781774087075\",\n  \"type\": \"apartment\",\n  \"address\": \"Lease Short St 1781774087075\",\n  \"description\": \"E2E reminders lease short property\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "9d1dbd92-ec0f-43c7-ac26-45a123ff723f",
            "date": "Thu, 18 Jun 2026 09:14:47 GMT",
            "content-length": "329"
          },
          "data": {
            "address": "Lease Short St 1781774087075",
            "created_at": "2026-06-18T12:14:47.078443+03:00",
            "description": "E2E reminders lease short property",
            "id": "a220f042-ac35-4bf8-90b1-2fdd4aaf898a",
            "name": "E2E Lease Short 1781774087075",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:47.078443+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 7,
          "duration": 7,
          "size": 329
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "0HLNpvb2g9LOWp-exXtPZ"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "9FOkK6QfbkP9Rofhh9TsR"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.169633291,
        "name": "Create Property Lease Short",
        "path": "reminders-e2e/00-setup/create property lease short",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create lease short.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"property_id\": \"a220f042-ac35-4bf8-90b1-2fdd4aaf898a\",\n  \"tenant_contact_id\": \"2cdbdd17-cef9-46f9-9134-e43d0ecee37e\",\n  \"start_date\": \"2026-06-18\",\n  \"end_date\": \"2026-07-03\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease short\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "141b6b2d-a2ad-4a9c-a76f-d56fac52849c",
            "date": "Thu, 18 Jun 2026 09:14:47 GMT",
            "content-length": "788"
          },
          "data": {
            "comment": "E2E lease short",
            "created_at": "2026-06-18T12:14:47.253904+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-03",
            "id": "63e50dc6-6711-4554-a04f-bd923683da64",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "a220f042-ac35-4bf8-90b1-2fdd4aaf898a",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:47.253904+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 9,
          "duration": 9,
          "size": 788
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "DUB7advHkEoJA5Cju8TAV"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "YUu8e5lZ42z-c5xwMo6i5"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.190989042,
        "name": "Create Lease Short",
        "path": "reminders-e2e/00-setup/create lease short",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create property lease past.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"name\": \"E2E Lease Past 1781774087432\",\n  \"type\": \"apartment\",\n  \"address\": \"Lease Past St 1781774087432\",\n  \"description\": \"E2E reminders lease past property\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "f94391b9-afba-4f8b-a795-8a5dcfdf2f95",
            "date": "Thu, 18 Jun 2026 09:14:47 GMT",
            "content-length": "326"
          },
          "data": {
            "address": "Lease Past St 1781774087432",
            "created_at": "2026-06-18T12:14:47.436203+03:00",
            "description": "E2E reminders lease past property",
            "id": "80f2415d-4637-4053-b393-6e61545d5e45",
            "name": "E2E Lease Past 1781774087432",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:47.436203+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 13,
          "duration": 13,
          "size": 326
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "7zuEwcRIxhR6c7Oo1Vkbu"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "OtN_4-Ecse3_SbU7WbZQe"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.168081667,
        "name": "Create Property Lease Past",
        "path": "reminders-e2e/00-setup/create property lease past",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create lease past.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"property_id\": \"80f2415d-4637-4053-b393-6e61545d5e45\",\n  \"tenant_contact_id\": \"2cdbdd17-cef9-46f9-9134-e43d0ecee37e\",\n  \"start_date\": \"2026-05-19\",\n  \"end_date\": \"2026-06-13\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease past\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "61a6c87f-cf03-4e83-bfc4-248f2d9dbea9",
            "date": "Thu, 18 Jun 2026 09:14:47 GMT",
            "content-length": "796"
          },
          "data": {
            "comment": "E2E lease past",
            "created_at": "2026-06-18T12:14:47.606452+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-06-13",
            "id": "ade7779e-53db-486c-af4f-1465a64bc5b1",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "80f2415d-4637-4053-b393-6e61545d5e45",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-05-19",
            "status": "requires_action",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:47.606452+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 16,
          "duration": 16,
          "size": 796
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "1XJaK9zkOP9FEeD8LxlgY"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "bTr9pIiyVbJL_yFVSMkmE"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.175316667,
        "name": "Create Lease Past",
        "path": "reminders-e2e/00-setup/create lease past",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create property lifecycle.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"name\": \"E2E Lifecycle 1781774087772\",\n  \"type\": \"apartment\",\n  \"address\": \"Lifecycle St 1781774087772\",\n  \"description\": \"E2E reminders lifecycle property\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "8f667b26-d48d-4959-8eb9-b12de3ff5ae7",
            "date": "Thu, 18 Jun 2026 09:14:47 GMT",
            "content-length": "323"
          },
          "data": {
            "address": "Lifecycle St 1781774087772",
            "created_at": "2026-06-18T12:14:47.774921+03:00",
            "description": "E2E reminders lifecycle property",
            "id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
            "name": "E2E Lifecycle 1781774087772",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:47.774921+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 5,
          "duration": 5,
          "size": 323
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "3PTDT0j4A8GD3AETG8hAO"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "spWK_bsAH9gx6DgQNP3Ft"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.155098583,
        "name": "Create Property Lifecycle",
        "path": "reminders-e2e/00-setup/create property lifecycle",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create lease lifecycle.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"property_id\": \"bfcb66ed-ec8c-43d3-b512-bd915f69cc90\",\n  \"tenant_contact_id\": \"2cdbdd17-cef9-46f9-9134-e43d0ecee37e\",\n  \"start_date\": \"2026-06-18\",\n  \"end_date\": \"2026-07-18\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease lifecycle\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "1900f80c-e18f-42c1-80ac-7473bd8f07b8",
            "date": "Thu, 18 Jun 2026 09:14:47 GMT",
            "content-length": "790"
          },
          "data": {
            "comment": "E2E lease lifecycle",
            "created_at": "2026-06-18T12:14:47.93422+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-18",
            "id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:47.93422+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 9,
          "duration": 9,
          "size": 790
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "akQyCxsDGAeWKsEYx_ygz"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "tPVjqEMBdxJO9UiMHYXUZ"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.161331959,
        "name": "Create Lease Lifecycle",
        "path": "reminders-e2e/00-setup/create lease lifecycle",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create operation due today.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"income\",\n  \"category\": \"rent\",\n  \"amount_kopecks\": 150000,\n  \"operation_date\": \"2026-06-18\",\n  \"comment\": \"E2E operation due today\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "3bb99e6d-f10f-4d0d-871f-5813fa49323f",
            "date": "Thu, 18 Jun 2026 09:14:48 GMT",
            "content-length": "435"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E operation due today",
            "created_at": "2026-06-18T12:14:48.096411+03:00",
            "id": "5ac4fe26-644d-401b-86d2-cf609b5d9134",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-18",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T12:14:48.096411+03:00"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations",
          "responseTime": 4,
          "duration": 4,
          "size": 435
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "LVBTgr1Y29PYIzASzVlCq"
          },
          {
            "description": "should have operation id",
            "status": "pass",
            "uid": "Bd6z5ROYxKmS8KQO03yHE"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.154123917,
        "name": "Create Operation Due Today",
        "path": "reminders-e2e/00-setup/create operation due today",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create operation to delete.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"income\",\n  \"category\": \"rent\",\n  \"amount_kopecks\": 150000,\n  \"operation_date\": \"2026-06-25\",\n  \"comment\": \"E2E operation to delete\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "338375a5-7cab-4c3b-8385-a7c5ebf7e051",
            "date": "Thu, 18 Jun 2026 09:14:48 GMT",
            "content-length": "435"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E operation to delete",
            "created_at": "2026-06-18T12:14:48.245079+03:00",
            "id": "1daccdfa-3068-4cc8-b4e9-ab840797f713",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-25",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T12:14:48.245079+03:00"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations",
          "responseTime": 4,
          "duration": 4,
          "size": 435
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "0jvrVBVNDOHL_QeYTgIkj"
          },
          {
            "description": "should have operation id",
            "status": "pass",
            "uid": "6TlKiPg2mk5hD0qLEp86N"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.14963525,
        "name": "Create Operation To Delete",
        "path": "reminders-e2e/00-setup/create operation to delete",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create recurring operation no future.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"expense\",\n  \"category\": \"utilities\",\n  \"amount_kopecks\": 50000,\n  \"start_date\": \"2026-05-19\",\n  \"end_date\": \"2026-06-08\",\n  \"payment_day\": 5,\n  \"comment\": \"E2E recurring operation no future\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "984a2f3d-f4bd-4288-b9fb-6a8709559c5f",
            "date": "Thu, 18 Jun 2026 09:14:48 GMT",
            "content-length": "478"
          },
          "data": {
            "amount_kopecks": 50000,
            "category": "utilities",
            "comment": "E2E recurring operation no future",
            "created_at": "2026-06-18T12:14:48.394113+03:00",
            "end_date": "2026-06-08",
            "id": "163c42c3-fe91-45bd-b462-2760499f5746",
            "lease_id": null,
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 5,
            "periodicity": "monthly",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "start_date": "2026-05-19",
            "status": "active",
            "type": "expense",
            "updated_at": "2026-06-18T12:14:48.394113+03:00"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations",
          "responseTime": 6,
          "duration": 6,
          "size": 478
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "8Jb1nrPDktBt_BHzicEnm"
          },
          {
            "description": "should have recurring operation id",
            "status": "pass",
            "uid": "6rH0tfaAwsHM051XJAJvW"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.152737917,
        "name": "Create Recurring Operation No Future",
        "path": "reminders-e2e/00-setup/create recurring operation no future",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/create recurring operation offset test.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"expense\",\n  \"category\": \"utilities\",\n  \"amount_kopecks\": 50000,\n  \"start_date\": \"2026-06-21\",\n  \"payment_day\": 21,\n  \"comment\": \"E2E recurring operation offset test\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "0a81b8c2-7358-4e38-a622-3eb0d8e26544",
            "date": "Thu, 18 Jun 2026 09:14:48 GMT",
            "content-length": "473"
          },
          "data": {
            "amount_kopecks": 50000,
            "category": "utilities",
            "comment": "E2E recurring operation offset test",
            "created_at": "2026-06-18T12:14:48.547224+03:00",
            "end_date": null,
            "id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
            "lease_id": null,
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 21,
            "periodicity": "monthly",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "start_date": "2026-06-21",
            "status": "active",
            "type": "expense",
            "updated_at": "2026-06-18T12:14:48.547224+03:00"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations",
          "responseTime": 10,
          "duration": 10,
          "size": 473
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "7on47DlAfua1dUJIiH_R0"
          },
          {
            "description": "should have recurring operation id",
            "status": "pass",
            "uid": "Io9ey6xIMEFOskG6yCZfF"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.151484125,
        "name": "Create Recurring Operation Offset Test",
        "path": "reminders-e2e/00-setup/create recurring operation offset test",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/verify lease 30d reminders.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/leases/5f37025f-f1bb-472b-b31e-e6c9fb449e17/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "e1060d05-9928-405c-9172-28172cf13956",
            "date": "Thu, 18 Jun 2026 09:14:48 GMT",
            "content-length": "1418"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0b03c3b7-e781-4f13-a9fd-6bb47c2d153e",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Аренда по объекту заканчивается 18.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "0d4aa99e-538f-4a47-8906-d96d60364c5a",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/5f37025f-f1bb-472b-b31e-e6c9fb449e17/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 1418
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "fDGaoFaGMcqlpQv6FVDlq"
          },
          {
            "description": "should contain lease_expiring reminder",
            "status": "pass",
            "uid": "dvlaJL2jjPApj-TqMSSVB"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "4YmG4SXxQlVar3IHxWWeO"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.147317375,
        "name": "Verify Lease 30d Reminders",
        "path": "reminders-e2e/00-setup/verify lease 30d reminders",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/verify lease short reminders.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/leases/63e50dc6-6711-4554-a04f-bd923683da64/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "3c798867-af5b-432b-bddf-0eddca4bf30f",
            "date": "Thu, 18 Jun 2026 09:14:48 GMT",
            "content-length": "744"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:47.256962+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "681bd171-e238-451a-96e1-27ee293f84e0",
                "lease_id": "63e50dc6-6711-4554-a04f-bd923683da64",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "a220f042-ac35-4bf8-90b1-2fdd4aaf898a",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-04T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.253904+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/63e50dc6-6711-4554-a04f-bd923683da64/reminders",
          "responseTime": 5,
          "duration": 5,
          "size": 744
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "4Mvhj6AYGKHW_BFcmKmDi"
          },
          {
            "description": "should not contain lease_expiring reminder",
            "status": "pass",
            "uid": "_6sRKxNW6oM8zl3NSsd7M"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "PhhYHQi9CtkHU47FcCoil"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.100268958,
        "name": "Verify Lease Short Reminders",
        "path": "reminders-e2e/00-setup/verify lease short reminders",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/00-setup/verify lease past reminders.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/leases/ade7779e-53db-486c-af4f-1465a64bc5b1/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "3a84287c-96dd-4c9e-a93c-168f710467a7",
            "date": "Thu, 18 Jun 2026 09:14:48 GMT",
            "content-length": "744"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:47.610842+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "a156e582-83b7-4c61-b09b-d1ab5fdf5233",
                "lease_id": "ade7779e-53db-486c-af4f-1465a64bc5b1",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "80f2415d-4637-4053-b393-6e61545d5e45",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.606452+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/ade7779e-53db-486c-af4f-1465a64bc5b1/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 744
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "ul2nTLov_G3Enmk4S3muw"
          },
          {
            "description": "should not contain lease_expiring reminder",
            "status": "pass",
            "uid": "jXrIzqElc8fO4H7PMRolS"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "gjg_0YWV8PMrlaMJcgXd9"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.149882666,
        "name": "Verify Lease Past Reminders",
        "path": "reminders-e2e/00-setup/verify lease past reminders",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/01-positive/create operation reminder.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-23\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "ec481613-c565-4dba-b8df-e18a650d6d48",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-18T09:14:49.098311Z",
            "event_type": "operation_due",
            "failed_attempts": 0,
            "id": "dee0ff23-218f-4573-93b5-957097637082",
            "lease_id": null,
            "message_body": "rent 1500.00 ₽ запланировано на 25.06.2026",
            "message_title": "Напоминание об операции",
            "next_attempt_at": null,
            "operation_id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "scheduled_at": "2026-06-23T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-18T09:14:49.098311Z"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651/reminders",
          "responseTime": 7,
          "duration": 7,
          "size": 645
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "JfkEhmzTAbqHQMTBB_IoF"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "cckU3c03xRv77giRWdVof"
          },
          {
            "description": "should be pending",
            "status": "pass",
            "uid": "XzqTlKhnMcUZChzQfMXmL"
          },
          {
            "description": "should be operation target",
            "status": "pass",
            "uid": "A020DrbEkLFeW6J8jvoiS"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.151247834,
        "name": "Create Operation Reminder",
        "path": "reminders-e2e/01-positive/create operation reminder",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/01-positive/get operation reminders.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "aa704bfb-4ce8-4435-a8d1-8ddea1ec7dda",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "content-length": "672"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:49.098311+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "dee0ff23-218f-4573-93b5-957097637082",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 25.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-23T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.098987+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651/reminders",
          "responseTime": 5,
          "duration": 5,
          "size": 672
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "TgjFSC2HsPOMK-9ZvxLwQ"
          },
          {
            "description": "should contain created reminder",
            "status": "pass",
            "uid": "zT3iGzZlPz85_F3aDd5fk"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.102536542,
        "name": "Get Operation Reminders",
        "path": "reminders-e2e/01-positive/get operation reminders",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/01-positive/create recurring operation reminder.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/86be2053-3b91-48e5-9cab-c38b0b41e1f4/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-21\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "33988e58-2e23-4924-af30-a7b2b4ae325c",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "b51ab2af-94c9-471c-a5d2-52b53c46f0cc",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c68ce1a3-0006-4ac6-b788-f1359473942e",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-06-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2e76fff0-f790-483b-94ff-21c4281da8a9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5435cef-cc5a-4321-923f-4ed226171216",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-07-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "eaa4d526-71bc-40b0-bfcb-056558599901",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac4498a8-ffee-43b3-9a2c-2fa8843ef791",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-08-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "05a3c835-fedf-455a-b80d-fba0d7077ab8",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f0c7f7a2-5935-4684-b386-af42cc958280",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-09-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "85dde5ab-37fe-4744-a702-646512bce643",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "e1ab8e62-16cb-42bb-adc5-3d95870b8343",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-10-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "8a2da950-eaaa-477d-ab82-d8dbfc66da81",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35f0e8be-a587-4594-a27a-d1fe9b5ab914",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-11-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0ce8f3c8-c008-4440-95f3-fb36a6cba80c",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3c0dc781-4f73-4b0e-85d4-a776fc6b6c65",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-12-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e6645257-922c-46e6-830e-fab41ad66ba2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6365b747-e70e-4ec0-a577-0091a9bd2cb3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-01-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7a711018-dcc2-44e2-857f-43d466a68919",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac81d256-f2a9-44b5-8a75-5585fb491b5a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-02-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "26acf060-c0e2-4f64-8a9c-0e18cbc50b5d",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d5426584-0b6f-420c-b566-d8512536ae59",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-03-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0b76f8c4-fcc9-4f79-bede-935c92f0f912",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "559b72ff-62b7-4990-bb98-c0fb2eebbdae",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-04-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "93afc9a5-9cf3-48f4-b3f8-d02857355111",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2e94cf18-c6b6-45df-81ce-d924977d2fe1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-05-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/86be2053-3b91-48e5-9cab-c38b0b41e1f4/reminders",
          "responseTime": 23,
          "duration": 23,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "Iuto-lKLrDOLiI83dntsh"
          },
          {
            "description": "should return array of reminders",
            "status": "pass",
            "uid": "ukaVKaxe88mxgcFxy5Wxm"
          },
          {
            "description": "should have reminder ids",
            "status": "pass",
            "uid": "srMtQqGiEk6R8TJh70GvN"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.17826975,
        "name": "Create Recurring Operation Reminder",
        "path": "reminders-e2e/01-positive/create recurring operation reminder",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/01-positive/get recurring operation reminders.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/86be2053-3b91-48e5-9cab-c38b0b41e1f4/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "5c5f9340-cbec-4b8a-a211-d3bf851332df",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "b51ab2af-94c9-471c-a5d2-52b53c46f0cc",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c68ce1a3-0006-4ac6-b788-f1359473942e",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-06-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2e76fff0-f790-483b-94ff-21c4281da8a9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5435cef-cc5a-4321-923f-4ed226171216",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-07-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "eaa4d526-71bc-40b0-bfcb-056558599901",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac4498a8-ffee-43b3-9a2c-2fa8843ef791",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-08-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "05a3c835-fedf-455a-b80d-fba0d7077ab8",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f0c7f7a2-5935-4684-b386-af42cc958280",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-09-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "85dde5ab-37fe-4744-a702-646512bce643",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "e1ab8e62-16cb-42bb-adc5-3d95870b8343",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-10-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "8a2da950-eaaa-477d-ab82-d8dbfc66da81",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35f0e8be-a587-4594-a27a-d1fe9b5ab914",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-11-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0ce8f3c8-c008-4440-95f3-fb36a6cba80c",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3c0dc781-4f73-4b0e-85d4-a776fc6b6c65",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-12-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e6645257-922c-46e6-830e-fab41ad66ba2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6365b747-e70e-4ec0-a577-0091a9bd2cb3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-01-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7a711018-dcc2-44e2-857f-43d466a68919",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac81d256-f2a9-44b5-8a75-5585fb491b5a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-02-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "26acf060-c0e2-4f64-8a9c-0e18cbc50b5d",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d5426584-0b6f-420c-b566-d8512536ae59",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-03-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0b76f8c4-fcc9-4f79-bede-935c92f0f912",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "559b72ff-62b7-4990-bb98-c0fb2eebbdae",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-04-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "93afc9a5-9cf3-48f4-b3f8-d02857355111",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2e94cf18-c6b6-45df-81ce-d924977d2fe1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-05-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/86be2053-3b91-48e5-9cab-c38b0b41e1f4/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "Tg5sZioCEHWrANIzp4sSu"
          },
          {
            "description": "should contain created reminders",
            "status": "pass",
            "uid": "1JC8obXCeqPG-rQlSL7Kv"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.102838917,
        "name": "Get Recurring Operation Reminders",
        "path": "reminders-e2e/01-positive/get recurring operation reminders",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/01-positive/get lease reminders.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/leases/5f37025f-f1bb-472b-b31e-e6c9fb449e17/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "f1340b48-5e8f-490a-b4c5-1d396073447e",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "content-length": "1418"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0b03c3b7-e781-4f13-a9fd-6bb47c2d153e",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Аренда по объекту заканчивается 18.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "0d4aa99e-538f-4a47-8906-d96d60364c5a",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/5f37025f-f1bb-472b-b31e-e6c9fb449e17/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 1418
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "kGmSiJerGSPmx46SbY8fC"
          },
          {
            "description": "should return array",
            "status": "pass",
            "uid": "kbrkUeWoeHxH4_EtsgvED"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.100359042,
        "name": "Get Lease Reminders",
        "path": "reminders-e2e/01-positive/get lease reminders",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/01-positive/list reminders default pagination.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "d745a22f-1826-4cd4-94f9-91ce134d8229",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:08.66977+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0cca588e-8ef4-439d-af70-db2e26e8b00e",
                "lease_id": "eb03273a-e10a-4fac-b9c0-b98e27517527",
                "message_body": "Аренда по объекту заканчивается 17.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.213612+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:02.225926+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "57ae1266-8252-49d4-a5e7-d78a3a5ee920",
                "lease_id": "e873f094-d8eb-4736-a45c-586e9ef7833f",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.826213+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.268705+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "38a9f21b-d053-4adb-bc4b-2bcc400ebf68",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 17.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "36495d17-732b-4016-8d2c-4eecdbd46bf1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": "2026-06-18T02:45:46.867408+03:00",
                "status": "sent",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:46.880976+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:02.603096+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "affa1b6d-840d-4b01-a768-27b33eb2c3f4",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Аренда по объекту заканчивается 17.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:08.174052+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:09.032479+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "559ad8b7-62e5-4878-8cd8-4b635e2cd9f2",
                "lease_id": null,
                "message_body": "rent 50000.00 ₽ запланировано на 17.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fcab6765-4407-4d4b-b5b8-ec8bba6e9e8a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
                "recurring_operation_id": "326693b1-9ba5-42a2-b772-7a6e68f967af",
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:09.213612+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:01.481915+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0fc878bb-6332-4059-92c4-e64a40f1c6be",
                "lease_id": "f8f3df33-2fa4-4aa4-ad97-7372eb9d73df",
                "message_body": "Аренда по объекту заканчивается 17.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.576157+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0b03c3b7-e781-4f13-a9fd-6bb47c2d153e",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Аренда по объекту заканчивается 18.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.610842+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "a156e582-83b7-4c61-b09b-d1ab5fdf5233",
                "lease_id": "ade7779e-53db-486c-af4f-1465a64bc5b1",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "80f2415d-4637-4053-b393-6e61545d5e45",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.606452+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.937604+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "3e4d27a0-0177-4326-96dd-391c8f528a71",
                "lease_id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
                "message_body": "Аренда по объекту заканчивается 18.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.93422+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "b51ab2af-94c9-471c-a5d2-52b53c46f0cc",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c68ce1a3-0006-4ac6-b788-f1359473942e",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-06-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.758107+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d0b83338-328c-4185-90dc-626d8dd387ad",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 24.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "e345ebe2-3958-4d12-8ec0-47ca8064927c",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-22T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.933328+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:05.613059+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d5297052-c047-41c8-9507-d5fa7fdc7d00",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 17.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "36495d17-732b-4016-8d2c-4eecdbd46bf1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-22T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:05.787451+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.098311+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "dee0ff23-218f-4573-93b5-957097637082",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 25.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-23T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.098987+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.038046+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "53ee5608-6397-4f55-a9e7-14b7e63a45e0",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 24.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "9b53d867-6cd6-440e-b63a-02a8c63a53db",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-27T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.506876+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.512941+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ac4abc2f-e5a1-454d-b1ea-9409e02fd208",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 27.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "9b53d867-6cd6-440e-b63a-02a8c63a53db",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-27T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.506876+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:08.1838+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "880aae38-2de1-47e5-87b9-c79e47c019b3",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Аренда по объекту заканчивается 01.08.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-02T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.950693+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:01.850927+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "da9ff133-b156-469e-ad21-e15243f14cd1",
                "lease_id": "e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "44bc828a-fd1e-410c-95c7-73fc77034005",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-03T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.701155+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.256962+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "681bd171-e238-451a-96e1-27ee293f84e0",
                "lease_id": "63e50dc6-6711-4554-a04f-bd923683da64",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "a220f042-ac35-4bf8-90b1-2fdd4aaf898a",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-04T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.253904+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:08.66977+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "96120483-d2cf-4321-aa20-55e389a58975",
                "lease_id": "eb03273a-e10a-4fac-b9c0-b98e27517527",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.213612+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:02.603096+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "a4d16801-644b-4c50-bd73-d653a38af2d9",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:08.174052+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:01.481915+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "b0064585-dc75-4681-9c74-d4052b28ff4a",
                "lease_id": "f8f3df33-2fa4-4aa4-ad97-7372eb9d73df",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.576157+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.937604+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "f6fc3f2b-b9e3-4836-9459-d215e2ec87d3",
                "lease_id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.93422+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "0d4aa99e-538f-4a47-8906-d96d60364c5a",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2e76fff0-f790-483b-94ff-21c4281da8a9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5435cef-cc5a-4321-923f-4ed226171216",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-07-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:08.1838+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "65b29dc3-d470-4ab3-a3c5-e4adda583f17",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-08-02T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.950693+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "eaa4d526-71bc-40b0-bfcb-056558599901",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac4498a8-ffee-43b3-9a2c-2fa8843ef791",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-08-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "05a3c835-fedf-455a-b80d-fba0d7077ab8",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f0c7f7a2-5935-4684-b386-af42cc958280",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-09-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "85dde5ab-37fe-4744-a702-646512bce643",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "e1ab8e62-16cb-42bb-adc5-3d95870b8343",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-10-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "8a2da950-eaaa-477d-ab82-d8dbfc66da81",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35f0e8be-a587-4594-a27a-d1fe9b5ab914",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-11-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0ce8f3c8-c008-4440-95f3-fb36a6cba80c",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3c0dc781-4f73-4b0e-85d4-a776fc6b6c65",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-12-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e6645257-922c-46e6-830e-fab41ad66ba2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6365b747-e70e-4ec0-a577-0091a9bd2cb3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-01-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7a711018-dcc2-44e2-857f-43d466a68919",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac81d256-f2a9-44b5-8a75-5585fb491b5a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-02-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "26acf060-c0e2-4f64-8a9c-0e18cbc50b5d",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d5426584-0b6f-420c-b566-d8512536ae59",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-03-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0b76f8c4-fcc9-4f79-bede-935c92f0f912",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "559b72ff-62b7-4990-bb98-c0fb2eebbdae",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-04-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "93afc9a5-9cf3-48f4-b3f8-d02857355111",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2e94cf18-c6b6-45df-81ce-d924977d2fe1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-05-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              }
            ]
          },
          "url": "http://localhost/reminders",
          "responseTime": 6,
          "duration": 6,
          "size": 24454
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "eKW3hRXevvKeFMJjvi-L1"
          },
          {
            "description": "should return array",
            "status": "pass",
            "uid": "ndxCtTbQy07-ewmuG9jwf"
          },
          {
            "description": "should include created reminders",
            "status": "pass",
            "uid": "FQBTh_5mzIPbKjAeF05FG"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.101703333,
        "name": "List Reminders Default Pagination",
        "path": "reminders-e2e/01-positive/list reminders default pagination",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/01-positive/update reminder scheduled at.bru"
        },
        "request": {
          "method": "PATCH",
          "url": "http://localhost:8080/reminders/dee0ff23-218f-4573-93b5-957097637082",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-28\"\n}"
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "3f226439-97f1-4a7c-b01a-450b491ca8ad",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "content-length": "655"
          },
          "data": {
            "created_at": "2026-06-18T12:14:49.098311+03:00",
            "event_type": "operation_due",
            "failed_attempts": 0,
            "id": "dee0ff23-218f-4573-93b5-957097637082",
            "lease_id": null,
            "message_body": "rent 1500.00 ₽ запланировано на 25.06.2026",
            "message_title": "Напоминание об операции",
            "next_attempt_at": null,
            "operation_id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "scheduled_at": "2026-06-28T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-18T12:14:49.098987+03:00"
          },
          "url": "http://localhost/reminders/dee0ff23-218f-4573-93b5-957097637082",
          "responseTime": 7,
          "duration": 7,
          "size": 655
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "8u9NJGt31J08cUTkLfWBa"
          },
          {
            "description": "should have same id",
            "status": "pass",
            "uid": "ked1TOX5yd28KeAtxW3GT"
          },
          {
            "description": "should still be pending",
            "status": "pass",
            "uid": "nlFSW66CwHQdrooRkxttl"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.102444125,
        "name": "Update Reminder Scheduled At",
        "path": "reminders-e2e/01-positive/update reminder scheduled at",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/no auth cookie.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/reminders",
          "headers": {
            "content-type": null
          }
        },
        "response": {
          "status": 401,
          "statusText": "Unauthorized",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "4117a124-4b9f-4be0-8824-7dfca49e4b5a",
            "date": "Thu, 18 Jun 2026 09:14:49 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "session required",
            "requestId": "4117a124-4b9f-4be0-8824-7dfca49e4b5a",
            "status": 401,
            "title": "Unauthorized",
            "type": "about:blank"
          },
          "url": "http://localhost/reminders",
          "responseTime": 1,
          "duration": 1,
          "size": 138
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 401",
            "status": "pass",
            "uid": "P1tLXhl4cQGJmBomG83_V"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.096013667,
        "name": "No Auth Cookie",
        "path": "reminders-e2e/02-negative/no auth cookie",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/reminder for non-existent operation.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/00000000-0000-0000-0000-000000000000/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-23\"\n}"
        },
        "response": {
          "status": 404,
          "statusText": "Not Found",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "52ede516-edaa-465a-b482-5e1070fdf2a8",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "operation not found",
            "requestId": "52ede516-edaa-465a-b482-5e1070fdf2a8",
            "status": 404,
            "title": "Not found",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/00000000-0000-0000-0000-000000000000/reminders",
          "responseTime": 3,
          "duration": 3,
          "size": 138
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 404",
            "status": "pass",
            "uid": "w1eWF3Ne8PnbwXIcSIO2r"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.099253958,
        "name": "Reminder For Non Existent Operation",
        "path": "reminders-e2e/02-negative/reminder for non-existent operation",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/reminder for another users resource.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/11111111-1111-1111-1111-111111111111/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-23\"\n}"
        },
        "response": {
          "status": 404,
          "statusText": "Not Found",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "4d39b06e-8266-44da-a3c5-a24f4a740a27",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "operation not found",
            "requestId": "4d39b06e-8266-44da-a3c5-a24f4a740a27",
            "status": 404,
            "title": "Not found",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/11111111-1111-1111-1111-111111111111/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 138
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 404",
            "status": "pass",
            "uid": "SehChS8mLOxS4bIf-Lqx3"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.09940925,
        "name": "Reminder For Another Users Resource",
        "path": "reminders-e2e/02-negative/reminder for another users resource",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/recurring reminder date in past.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/86be2053-3b91-48e5-9cab-c38b0b41e1f4/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-17\"\n}"
        },
        "response": {
          "status": 400,
          "statusText": "Bad Request",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "fecc89b9-f3dd-4fd3-be8a-09d824171cea",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "165"
          },
          "data": {
            "detail": "reminder date must be today or in the future",
            "requestId": "fecc89b9-f3dd-4fd3-be8a-09d824171cea",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/86be2053-3b91-48e5-9cab-c38b0b41e1f4/reminders",
          "responseTime": 6,
          "duration": 6,
          "size": 165
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "Zb2izTQhYZd0rep4rFXzO"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.105709875,
        "name": "Recurring Reminder Date In Past",
        "path": "reminders-e2e/02-negative/recurring reminder date in past",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/recurring reminder no future operations.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/163c42c3-fe91-45bd-b462-2760499f5746/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-23\"\n}"
        },
        "response": {
          "status": 400,
          "statusText": "Bad Request",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "2ff58a61-c1ad-4a60-977b-52e0602f5b9c",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "154"
          },
          "data": {
            "detail": "no future operations for reminder",
            "requestId": "2ff58a61-c1ad-4a60-977b-52e0602f5b9c",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/163c42c3-fe91-45bd-b462-2760499f5746/reminders",
          "responseTime": 6,
          "duration": 6,
          "size": 154
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "7434HaVYiof_17kExLSyJ"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.10158525,
        "name": "Recurring Reminder No Future Operations",
        "path": "reminders-e2e/02-negative/recurring reminder no future operations",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/create reminder for cancel test.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/5ac4fe26-644d-401b-86d2-cf609b5d9134/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-23\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "d908bd92-cf51-4ce7-80de-2077e89bedcb",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-18T09:14:50.439428Z",
            "event_type": "operation_due",
            "failed_attempts": 0,
            "id": "88d7c779-610b-4002-b4eb-1f3c55cd550e",
            "lease_id": null,
            "message_body": "rent 1500.00 ₽ запланировано на 18.06.2026",
            "message_title": "Напоминание об операции",
            "next_attempt_at": null,
            "operation_id": "5ac4fe26-644d-401b-86d2-cf609b5d9134",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "scheduled_at": "2026-06-23T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-18T09:14:50.439428Z"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/5ac4fe26-644d-401b-86d2-cf609b5d9134/reminders",
          "responseTime": 7,
          "duration": 7,
          "size": 645
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "Iod5ktpJZjl-r3NBrUmkr"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "l7JxxCa-dbGXz7lMoYzrl"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.149834042,
        "name": "Create Reminder For Cancel Test",
        "path": "reminders-e2e/02-negative/create reminder for cancel test",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/delete reminder for cancel test.bru"
        },
        "request": {
          "method": "DELETE",
          "url": "http://localhost:8080/reminders/88d7c779-610b-4002-b4eb-1f3c55cd550e",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 204,
          "statusText": "No Content",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "0ddf4ddd-d6f9-4901-bfcf-7f07b7293d37",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT"
          },
          "data": "",
          "url": "http://localhost/reminders/88d7c779-610b-4002-b4eb-1f3c55cd550e",
          "responseTime": 5,
          "duration": 5,
          "size": 0
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 204",
            "status": "pass",
            "uid": "7VC5kX7U7JzVqt4MALJ-5"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.113719083,
        "name": "Delete Reminder For Cancel Test",
        "path": "reminders-e2e/02-negative/delete reminder for cancel test",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/update cancelled reminder.bru"
        },
        "request": {
          "method": "PATCH",
          "url": "http://localhost:8080/reminders/88d7c779-610b-4002-b4eb-1f3c55cd550e",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-07-03\"\n}"
        },
        "response": {
          "status": 400,
          "statusText": "Bad Request",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "6d1f4205-bfcf-42cb-8b1a-b72b60b96cd8",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "144"
          },
          "data": {
            "detail": "reminder is not pending",
            "requestId": "6d1f4205-bfcf-42cb-8b1a-b72b60b96cd8",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/reminders/88d7c779-610b-4002-b4eb-1f3c55cd550e",
          "responseTime": 3,
          "duration": 3,
          "size": 144
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400 or 409",
            "status": "pass",
            "uid": "BBnaLrFEdbZUGcDd9MTzg"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.103775083,
        "name": "Update Cancelled Reminder",
        "path": "reminders-e2e/02-negative/update cancelled reminder",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/update non-existent reminder.bru"
        },
        "request": {
          "method": "PATCH",
          "url": "http://localhost:8080/reminders/00000000-0000-0000-0000-000000000000",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-23\"\n}"
        },
        "response": {
          "status": 404,
          "statusText": "Not Found",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "64257e5a-72d8-4b73-8e7c-9d760cff3bc8",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "137"
          },
          "data": {
            "detail": "reminder not found",
            "requestId": "64257e5a-72d8-4b73-8e7c-9d760cff3bc8",
            "status": 404,
            "title": "Not found",
            "type": "about:blank"
          },
          "url": "http://localhost/reminders/00000000-0000-0000-0000-000000000000",
          "responseTime": 3,
          "duration": 3,
          "size": 137
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 404",
            "status": "pass",
            "uid": "qLfe2IdlOktLZnKIVdXXl"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.098862583,
        "name": "Update Non Existent Reminder",
        "path": "reminders-e2e/02-negative/update non-existent reminder",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/02-negative/invalid limit offset.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/reminders?limit=abc&offset=def",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 400,
          "statusText": "Bad Request",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "37d1f485-e91f-4513-9eca-f27a88e7744f",
            "date": "Thu, 18 Jun 2026 09:14:50 GMT",
            "content-length": "238"
          },
          "data": {
            "detail": "Invalid format for parameter limit: error binding string parameter: strconv.ParseInt: parsing \"abc\": invalid syntax",
            "requestId": "37d1f485-e91f-4513-9eca-f27a88e7744f",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/reminders?limit=abc&offset=def",
          "responseTime": 3,
          "duration": 3,
          "size": 238
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "1qxQlfUwvP8UjTMw9zWqp"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.097756042,
        "name": "Invalid Limit Offset",
        "path": "reminders-e2e/02-negative/invalid limit offset",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/03-boundary/operation due today creates reminder today.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/5ac4fe26-644d-401b-86d2-cf609b5d9134/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-18\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "aea39e01-42c9-435e-800a-eb32878eee1c",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-18T09:14:51.004281Z",
            "event_type": "operation_due",
            "failed_attempts": 0,
            "id": "05a17292-5871-4ca1-9b6d-1706c2977f25",
            "lease_id": null,
            "message_body": "rent 1500.00 ₽ запланировано на 18.06.2026",
            "message_title": "Напоминание об операции",
            "next_attempt_at": null,
            "operation_id": "5ac4fe26-644d-401b-86d2-cf609b5d9134",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "scheduled_at": "2026-06-18T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-18T09:14:51.004281Z"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/5ac4fe26-644d-401b-86d2-cf609b5d9134/reminders",
          "responseTime": 6,
          "duration": 6,
          "size": 645
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "q2c5zKF6JM5i9fV2RpnGX"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "KSxJV2jwFuGmWQomKj7zX"
          },
          {
            "description": "should be pending",
            "status": "pass",
            "uid": "j7Y89FLQhARMPumO4sCIH"
          },
          {
            "description": "should be operation_due event",
            "status": "pass",
            "uid": "qnN4MGoqv4O_YzlXM9vZx"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.154075333,
        "name": "Operation Due Today Creates Reminder Today",
        "path": "reminders-e2e/03-boundary/operation due today creates reminder today",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/03-boundary/recurring reminder offset zero.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-19\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "15243804-5d2f-4059-b624-698d1028fa60",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1a3f7d2d-2f0c-4552-a0c7-2dfd0f5178c3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "083c1cbe-2558-476d-ae7d-c0db67c0e496",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-06-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7cf99dd1-cc32-49b3-91c9-05acbf82ee31",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "114444a3-71bc-4b66-a877-b5354fd85b12",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0bfec937-369b-40d3-a9a0-e6059d2c36b3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "76fc153a-77a2-48e7-85c5-cadd0350c04a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-08-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1cb6fadb-94ff-4ae8-a7a1-985c6b8ffc09",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "586fc3b0-7884-4c1a-8d54-8c7b5d7e8bc3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-09-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "9a764edf-122d-44af-87a5-73fad07c65c5",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da974028-9dd3-43cf-9e77-83098f20efaa",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-10-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "c47ebc12-7b08-4244-97cf-b7acf39d51ec",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22c81437-b70e-4ec7-bbf3-f657ede37755",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-11-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d82c712b-ac2a-4c49-8165-71d48e5683d3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd921b42-4584-455a-b42b-e35f5cbe1adf",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-12-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7caea7d0-a177-4d79-81c7-201f99fdf8e8",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c919d5e2-6dcb-41cc-87e3-63c4e26844b3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-01-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7958beca-a221-4c13-8507-49c73343ee47",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c7f463cb-5d95-49af-9877-8a56123132d7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-02-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e5dc3e46-65bf-4326-b169-00a6ed01d4ca",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "55398d6f-e8ae-46f0-a29a-2584b4ee1319",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-03-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "a45fac47-f797-42eb-88d4-7eb3dd1eb19b",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "8f887205-1829-4d4e-b805-b1e5055c15eb",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-04-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "84b6bf93-35cb-4feb-9398-b4d4c566e8ab",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "cc78ac8c-4d50-4647-8e8f-2151b9042856",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-05-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "responseTime": 17,
          "duration": 17,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "ZtjlIjB9JkecIAvMLEcaq"
          },
          {
            "description": "should return reminders",
            "status": "pass",
            "uid": "uvsOVQbP_Aa86WCjXjazH"
          },
          {
            "description": "reminders should be pending",
            "status": "pass",
            "uid": "-GuKwgUecPCUHPWUfjhW8"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.1603025,
        "name": "Recurring Reminder Offset Zero",
        "path": "reminders-e2e/03-boundary/recurring reminder offset zero",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/03-boundary/recurring reminder offset too large.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-08-17\"\n}"
        },
        "response": {
          "status": 400,
          "statusText": "Bad Request",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "4b257a6b-e891-4564-88ed-7e83a8d356b3",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "content-length": "190"
          },
          "data": {
            "detail": "reminder date must be on or before the earliest future operation date",
            "requestId": "4b257a6b-e891-4564-88ed-7e83a8d356b3",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "responseTime": 6,
          "duration": 6,
          "size": 190
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "--dUt14UzFAMHiswBynBt"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.1026725,
        "name": "Recurring Reminder Offset Too Large",
        "path": "reminders-e2e/03-boundary/recurring reminder offset too large",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/03-boundary/pagination limit zero.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/reminders?limit=0&offset=0",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "72346d2d-7e46-46b9-b7db-ac1b1842931f",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "content-length": "699"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:06.268705+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "38a9f21b-d053-4adb-bc4b-2bcc400ebf68",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 17.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "36495d17-732b-4016-8d2c-4eecdbd46bf1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": "2026-06-18T02:45:46.867408+03:00",
                "status": "sent",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:46.880976+03:00"
              }
            ]
          },
          "url": "http://localhost/reminders?limit=0&offset=0",
          "responseTime": 4,
          "duration": 4,
          "size": 699
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "v5LLOXSVxkoMdBjAw-fvx"
          },
          {
            "description": "should return array with at most one item",
            "status": "pass",
            "uid": "VP4Db_FeerZhNzMAyoNO9"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.098690875,
        "name": "Pagination Limit Zero",
        "path": "reminders-e2e/03-boundary/pagination limit zero",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/03-boundary/pagination limit large.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/reminders?limit=1000&offset=0",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "554628e8-1777-49a3-ad7a-6bcb8a064ce1",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:08.66977+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0cca588e-8ef4-439d-af70-db2e26e8b00e",
                "lease_id": "eb03273a-e10a-4fac-b9c0-b98e27517527",
                "message_body": "Аренда по объекту заканчивается 17.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.213612+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:01.481915+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0fc878bb-6332-4059-92c4-e64a40f1c6be",
                "lease_id": "f8f3df33-2fa4-4aa4-ad97-7372eb9d73df",
                "message_body": "Аренда по объекту заканчивается 17.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.576157+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.268705+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "38a9f21b-d053-4adb-bc4b-2bcc400ebf68",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 17.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "36495d17-732b-4016-8d2c-4eecdbd46bf1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": "2026-06-18T02:45:46.867408+03:00",
                "status": "sent",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:46.880976+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:09.032479+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "559ad8b7-62e5-4878-8cd8-4b635e2cd9f2",
                "lease_id": null,
                "message_body": "rent 50000.00 ₽ запланировано на 17.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fcab6765-4407-4d4b-b5b8-ec8bba6e9e8a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
                "recurring_operation_id": "326693b1-9ba5-42a2-b772-7a6e68f967af",
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:09.213612+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:02.603096+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "affa1b6d-840d-4b01-a768-27b33eb2c3f4",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Аренда по объекту заканчивается 17.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:08.174052+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:02.225926+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "57ae1266-8252-49d4-a5e7-d78a3a5ee920",
                "lease_id": "e873f094-d8eb-4736-a45c-586e9ef7833f",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-17T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.826213+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.004281+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "05a17292-5871-4ca1-9b6d-1706c2977f25",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 18.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "5ac4fe26-644d-401b-86d2-cf609b5d9134",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.003991+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "0b03c3b7-e781-4f13-a9fd-6bb47c2d153e",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Аренда по объекту заканчивается 18.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.610842+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "a156e582-83b7-4c61-b09b-d1ab5fdf5233",
                "lease_id": "ade7779e-53db-486c-af4f-1465a64bc5b1",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "80f2415d-4637-4053-b393-6e61545d5e45",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.606452+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.937604+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "3e4d27a0-0177-4326-96dd-391c8f528a71",
                "lease_id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
                "message_body": "Аренда по объекту заканчивается 18.07.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.93422+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1a3f7d2d-2f0c-4552-a0c7-2dfd0f5178c3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "083c1cbe-2558-476d-ae7d-c0db67c0e496",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-06-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "b51ab2af-94c9-471c-a5d2-52b53c46f0cc",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c68ce1a3-0006-4ac6-b788-f1359473942e",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-06-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:05.613059+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d5297052-c047-41c8-9507-d5fa7fdc7d00",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 17.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "36495d17-732b-4016-8d2c-4eecdbd46bf1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-22T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:05.787451+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.758107+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d0b83338-328c-4185-90dc-626d8dd387ad",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 24.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "e345ebe2-3958-4d12-8ec0-47ca8064927c",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-22T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.933328+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:50.439428+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "88d7c779-610b-4002-b4eb-1f3c55cd550e",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 18.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "5ac4fe26-644d-401b-86d2-cf609b5d9134",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-23T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:50.589742+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.512941+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ac4abc2f-e5a1-454d-b1ea-9409e02fd208",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 27.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "9b53d867-6cd6-440e-b63a-02a8c63a53db",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-27T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.506876+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.038046+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "53ee5608-6397-4f55-a9e7-14b7e63a45e0",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 24.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "9b53d867-6cd6-440e-b63a-02a8c63a53db",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-27T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.506876+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.098311+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "dee0ff23-218f-4573-93b5-957097637082",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 25.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-28T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.834987+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:08.1838+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "880aae38-2de1-47e5-87b9-c79e47c019b3",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Аренда по объекту заканчивается 01.08.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-02T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.950693+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:01.850927+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "da9ff133-b156-469e-ad21-e15243f14cd1",
                "lease_id": "e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "44bc828a-fd1e-410c-95c7-73fc77034005",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-03T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.701155+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.256962+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "681bd171-e238-451a-96e1-27ee293f84e0",
                "lease_id": "63e50dc6-6711-4554-a04f-bd923683da64",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "a220f042-ac35-4bf8-90b1-2fdd4aaf898a",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-04T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.253904+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:08.66977+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "96120483-d2cf-4321-aa20-55e389a58975",
                "lease_id": "eb03273a-e10a-4fac-b9c0-b98e27517527",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.213612+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:02.603096+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "a4d16801-644b-4c50-bd73-d653a38af2d9",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:08.174052+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:01.481915+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "b0064585-dc75-4681-9c74-d4052b28ff4a",
                "lease_id": "f8f3df33-2fa4-4aa4-ad97-7372eb9d73df",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.576157+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:47.937604+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "f6fc3f2b-b9e3-4836-9459-d215e2ec87d3",
                "lease_id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:47.93422+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:46.916376+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "0d4aa99e-538f-4a47-8906-d96d60364c5a",
                "lease_id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:46.910383+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7cf99dd1-cc32-49b3-91c9-05acbf82ee31",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "114444a3-71bc-4b66-a877-b5354fd85b12",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2e76fff0-f790-483b-94ff-21c4281da8a9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5435cef-cc5a-4321-923f-4ed226171216",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-07-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:08.1838+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "65b29dc3-d470-4ab3-a3c5-e4adda583f17",
                "lease_id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
                "recurring_operation_id": null,
                "scheduled_at": "2026-08-02T10:00:00+03:00",
                "sent_at": null,
                "status": "cancelled",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:09.950693+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0bfec937-369b-40d3-a9a0-e6059d2c36b3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "76fc153a-77a2-48e7-85c5-cadd0350c04a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-08-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "eaa4d526-71bc-40b0-bfcb-056558599901",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac4498a8-ffee-43b3-9a2c-2fa8843ef791",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-08-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1cb6fadb-94ff-4ae8-a7a1-985c6b8ffc09",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "586fc3b0-7884-4c1a-8d54-8c7b5d7e8bc3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-09-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "05a3c835-fedf-455a-b80d-fba0d7077ab8",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f0c7f7a2-5935-4684-b386-af42cc958280",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-09-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "9a764edf-122d-44af-87a5-73fad07c65c5",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da974028-9dd3-43cf-9e77-83098f20efaa",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-10-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "85dde5ab-37fe-4744-a702-646512bce643",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "e1ab8e62-16cb-42bb-adc5-3d95870b8343",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-10-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "c47ebc12-7b08-4244-97cf-b7acf39d51ec",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22c81437-b70e-4ec7-bbf3-f657ede37755",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-11-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "8a2da950-eaaa-477d-ab82-d8dbfc66da81",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35f0e8be-a587-4594-a27a-d1fe9b5ab914",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-11-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d82c712b-ac2a-4c49-8165-71d48e5683d3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd921b42-4584-455a-b42b-e35f5cbe1adf",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-12-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0ce8f3c8-c008-4440-95f3-fb36a6cba80c",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3c0dc781-4f73-4b0e-85d4-a776fc6b6c65",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2026-12-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7caea7d0-a177-4d79-81c7-201f99fdf8e8",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c919d5e2-6dcb-41cc-87e3-63c4e26844b3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-01-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e6645257-922c-46e6-830e-fab41ad66ba2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6365b747-e70e-4ec0-a577-0091a9bd2cb3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-01-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7958beca-a221-4c13-8507-49c73343ee47",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c7f463cb-5d95-49af-9877-8a56123132d7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-02-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7a711018-dcc2-44e2-857f-43d466a68919",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac81d256-f2a9-44b5-8a75-5585fb491b5a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-02-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e5dc3e46-65bf-4326-b169-00a6ed01d4ca",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "55398d6f-e8ae-46f0-a29a-2584b4ee1319",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-03-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "26acf060-c0e2-4f64-8a9c-0e18cbc50b5d",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d5426584-0b6f-420c-b566-d8512536ae59",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-03-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "a45fac47-f797-42eb-88d4-7eb3dd1eb19b",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "8f887205-1829-4d4e-b805-b1e5055c15eb",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-04-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "0b76f8c4-fcc9-4f79-bede-935c92f0f912",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "559b72ff-62b7-4990-bb98-c0fb2eebbdae",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-04-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.162521+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "84b6bf93-35cb-4feb-9398-b4d4c566e8ab",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "cc78ac8c-4d50-4647-8e8f-2151b9042856",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-05-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.159931+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:49.358856+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "93afc9a5-9cf3-48f4-b3f8-d02857355111",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2e94cf18-c6b6-45df-81ce-d924977d2fe1",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "86be2053-3b91-48e5-9cab-c38b0b41e1f4",
                "scheduled_at": "2027-05-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:49.354625+03:00"
              }
            ]
          },
          "url": "http://localhost/reminders?limit=1000&offset=0",
          "responseTime": 4,
          "duration": 4,
          "size": 34152
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "CfdiEjVWfnL-OCq9wNLle"
          },
          {
            "description": "should return array",
            "status": "pass",
            "uid": "HjcqTTZvyiho6_vg0fV2N"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.099496625,
        "name": "Pagination Limit Large",
        "path": "reminders-e2e/03-boundary/pagination limit large",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/03-boundary/pagination offset beyond count.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/reminders?limit=10&offset=100000",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "9d4a6fd1-1b16-4ea6-8365-29d983371694",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "content-length": "13"
          },
          "data": {
            "items": []
          },
          "url": "http://localhost/reminders?limit=10&offset=100000",
          "responseTime": 3,
          "duration": 3,
          "size": 13
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "9uGJTQPbX33I0Gr4IZ52j"
          },
          {
            "description": "should return empty array",
            "status": "pass",
            "uid": "Pe0_bewga0U9vBboe3uk7"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.098717541,
        "name": "Pagination Offset Beyond Count",
        "path": "reminders-e2e/03-boundary/pagination offset beyond count",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/create reminder for recurring offset change.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-19\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "735749a1-be36-47de-8eda-fb26af27b43a",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "196db215-7e88-4751-af34-1c627a9d328f",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "083c1cbe-2558-476d-ae7d-c0db67c0e496",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-06-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "aebc50e9-69d2-43bd-bc02-139cd7fa68b2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "114444a3-71bc-4b66-a877-b5354fd85b12",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-07-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "f735297a-9470-4737-b0dd-daa41c185a2f",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "76fc153a-77a2-48e7-85c5-cadd0350c04a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-08-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "92f624cd-15e9-4c4f-915d-0847d9997abb",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "586fc3b0-7884-4c1a-8d54-8c7b5d7e8bc3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-09-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2b3dabe0-fd0c-4454-a83e-898a81237e38",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da974028-9dd3-43cf-9e77-83098f20efaa",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-10-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ac1bd4de-c312-4e86-b2cd-4a1315c1f5cc",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22c81437-b70e-4ec7-bbf3-f657ede37755",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-11-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "42eda86a-5c61-4754-9e0a-756d27ada7a2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd921b42-4584-455a-b42b-e35f5cbe1adf",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-12-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d7e175d9-3c43-409e-9451-6d348ea9b3cc",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c919d5e2-6dcb-41cc-87e3-63c4e26844b3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-01-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "14464fcb-1bea-4c7e-b981-cbb53004d8b3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c7f463cb-5d95-49af-9877-8a56123132d7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-02-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "dde354d7-515e-4346-938a-ef1a1e18a824",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "55398d6f-e8ae-46f0-a29a-2584b4ee1319",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-03-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "21069999-4ebb-44ef-8bc6-22c7f1415198",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "8f887205-1829-4d4e-b805-b1e5055c15eb",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-04-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.721523+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "53c2fe6d-07b5-4cd6-b1d9-383551afa04d",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "cc78ac8c-4d50-4647-8e8f-2151b9042856",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-05-19T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.718515+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "responseTime": 18,
          "duration": 18,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "47ChQqhLTJbQfgREUgzos"
          },
          {
            "description": "should return reminders",
            "status": "pass",
            "uid": "yODoYLjplPkgA7to8GkFP"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.15925875,
        "name": "Create Reminder For Recurring Offset Change",
        "path": "reminders-e2e/04-lifecycle/create reminder for recurring offset change",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/update recurring operation offset.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-21\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "731582b5-c0eb-4e33-85ee-76304ee50d2e",
            "date": "Thu, 18 Jun 2026 09:14:51 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7d489a8c-ead8-4a6b-8384-f790dad21952",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "083c1cbe-2558-476d-ae7d-c0db67c0e496",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-06-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "24f6a707-9ee4-4e3f-a776-95428c1cc6de",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "114444a3-71bc-4b66-a877-b5354fd85b12",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-07-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "9b68f71d-226d-428d-9ac6-9078d65ab9e9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "76fc153a-77a2-48e7-85c5-cadd0350c04a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-08-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "fd242616-cc8c-408d-8c8e-6c85b33985c3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "586fc3b0-7884-4c1a-8d54-8c7b5d7e8bc3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-09-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "532cbf2b-1c5e-43d9-860e-ff7828e3a67f",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da974028-9dd3-43cf-9e77-83098f20efaa",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-10-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e3bed974-61a6-4344-bef3-e87f8a781270",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22c81437-b70e-4ec7-bbf3-f657ede37755",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-11-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "36c7ac5c-a196-47aa-bf24-f0c194228402",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd921b42-4584-455a-b42b-e35f5cbe1adf",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2026-12-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "a160e851-fba7-4947-b090-ee5694057555",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c919d5e2-6dcb-41cc-87e3-63c4e26844b3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-01-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "9e5f80ad-f1bc-4fc3-a7c8-823e75f3f163",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c7f463cb-5d95-49af-9877-8a56123132d7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-02-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "f37ad616-9cc0-4e06-aed6-41db72c82f6a",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "55398d6f-e8ae-46f0-a29a-2584b4ee1319",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-03-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "6d6da3c7-df81-460b-ab74-0cdf08c54b55",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "8f887205-1829-4d4e-b805-b1e5055c15eb",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-04-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:51.882823+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "9ecdf970-d024-431a-86bf-6268828ee70e",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 21.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "cc78ac8c-4d50-4647-8e8f-2151b9042856",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": "8eb71a27-3869-44c6-93e0-1999779b2ba4",
                "scheduled_at": "2027-05-21T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:51.879802+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/recurring-operations/8eb71a27-3869-44c6-93e0-1999779b2ba4/reminders",
          "responseTime": 18,
          "duration": 18,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "tDZUCO4xDeOdJCTjFqpp2"
          },
          {
            "description": "should return new reminders",
            "status": "pass",
            "uid": "i21rtnEWrfhl_jbl2jVSd"
          },
          {
            "description": "old reminders should be replaced",
            "status": "pass",
            "uid": "I_8bVGiFXXXZzOAH5m14K"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.161267834,
        "name": "Update Recurring Operation Offset",
        "path": "reminders-e2e/04-lifecycle/update recurring operation offset",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/update manual operation date.bru"
        },
        "request": {
          "method": "PATCH",
          "url": "http://localhost:8080/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"operation_date\": \"2026-06-28\",\n  \"comment\": \"E2E manual operation main updated\"\n}"
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "d6369e34-eeb5-4b1e-a87b-efec7cdb3ea4",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT",
            "content-length": "445"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E manual operation main updated",
            "created_at": "2026-06-18T12:14:46.434114+03:00",
            "id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-28",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T12:14:52.037125+03:00"
          },
          "url": "http://localhost/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651",
          "responseTime": 7,
          "duration": 7,
          "size": 445
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "ErW8BpfgF_B-k_kowHjzZ"
          },
          {
            "description": "should have updated date",
            "status": "pass",
            "uid": "zxpF2TzxCXr3J5bAwAh-y"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.103989375,
        "name": "Update Manual Operation Date",
        "path": "reminders-e2e/04-lifecycle/update manual operation date",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/verify operation reminder rescheduled.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "45cf3db6-05b4-45a2-aa41-b12b1548fed1",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT",
            "content-length": "672"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:52.040877+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ba747d4c-ba83-4b82-b058-75b11b7f3ec7",
                "lease_id": null,
                "message_body": "rent 1500.00 ₽ запланировано на 28.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "61d2c263-73a6-49d5-84ec-858d8ad1e651",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
                "recurring_operation_id": null,
                "scheduled_at": "2026-06-28T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:52.037125+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/61d2c263-73a6-49d5-84ec-858d8ad1e651/reminders",
          "responseTime": 3,
          "duration": 3,
          "size": 672
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "BtQAuOAfJYxPMi8UlHv9n"
          },
          {
            "description": "should have a pending reminder",
            "status": "pass",
            "uid": "ZxYKry-FFr6yzg7vVMoXl"
          },
          {
            "description": "reminder should be operation_due",
            "status": "pass",
            "uid": "QOTxgHUOc1qHXWF-H5ggz"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.10310225,
        "name": "Verify Operation Reminder Rescheduled",
        "path": "reminders-e2e/04-lifecycle/verify operation reminder rescheduled",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/create reminder to delete operation.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/1daccdfa-3068-4cc8-b4e9-ab840797f713/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-23\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "27d130f7-71c9-48ed-af7f-9f1b26de049a",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-18T09:14:52.244791Z",
            "event_type": "operation_due",
            "failed_attempts": 0,
            "id": "92d83a55-4d02-4ac2-b10c-efca3bacdf69",
            "lease_id": null,
            "message_body": "rent 1500.00 ₽ запланировано на 25.06.2026",
            "message_title": "Напоминание об операции",
            "next_attempt_at": null,
            "operation_id": "1daccdfa-3068-4cc8-b4e9-ab840797f713",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "recurring_operation_id": null,
            "scheduled_at": "2026-06-23T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-18T09:14:52.244791Z"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/1daccdfa-3068-4cc8-b4e9-ab840797f713/reminders",
          "responseTime": 5,
          "duration": 5,
          "size": 645
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "1ahi4O5C9VJeRwbC8TbbX"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "yqjeRGCZKIY3K32X2ZmZS"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.148434792,
        "name": "Create Reminder To Delete Operation",
        "path": "reminders-e2e/04-lifecycle/create reminder to delete operation",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/delete operation.bru"
        },
        "request": {
          "method": "DELETE",
          "url": "http://localhost:8080/operations/1daccdfa-3068-4cc8-b4e9-ab840797f713",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 204,
          "statusText": "No Content",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "b983f2ef-b519-4513-9eeb-5da14505d6c7",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT"
          },
          "data": "",
          "url": "http://localhost/operations/1daccdfa-3068-4cc8-b4e9-ab840797f713",
          "responseTime": 7,
          "duration": 7,
          "size": 0
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 204",
            "status": "pass",
            "uid": "jreOWBpD1oGND6VDA_L6m"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.104078084,
        "name": "Delete Operation",
        "path": "reminders-e2e/04-lifecycle/delete operation",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/verify operation reminders cancelled.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/1daccdfa-3068-4cc8-b4e9-ab840797f713/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 404,
          "statusText": "Not Found",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/problem+json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "a18e3bac-cc73-4c1f-97ba-9cd7c29d453e",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "operation not found",
            "requestId": "a18e3bac-cc73-4c1f-97ba-9cd7c29d453e",
            "status": 404,
            "title": "Not found",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/operations/1daccdfa-3068-4cc8-b4e9-ab840797f713/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 138
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 404 (operation deleted)",
            "status": "pass",
            "uid": "FC0QN5zw5NdsdwFPwntmw"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.098878125,
        "name": "Verify Operation Reminders Cancelled",
        "path": "reminders-e2e/04-lifecycle/verify operation reminders cancelled",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/update lease end date.bru"
        },
        "request": {
          "method": "PATCH",
          "url": "http://localhost:8080/leases/a2428b32-9bbf-4099-9938-2579fe8c65f1",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"end_date\": \"2026-08-02\",\n  \"comment\": \"E2E lease lifecycle updated\"\n}"
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "6141bad3-e8da-4796-9a08-17ed514d5c75",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT",
            "content-length": "799"
          },
          "data": {
            "comment": "E2E lease lifecycle updated",
            "created_at": "2026-06-18T12:14:47.93422+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-08-02",
            "id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:52.595486+03:00"
          },
          "url": "http://localhost/leases/a2428b32-9bbf-4099-9938-2579fe8c65f1",
          "responseTime": 17,
          "duration": 17,
          "size": 799
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "YlvyzKFN4OHi_zkrWdjSA"
          },
          {
            "description": "should have updated end date",
            "status": "pass",
            "uid": "Ze4GBLTq2p7wD5iXvBOhg"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.112623334,
        "name": "Update Lease End Date",
        "path": "reminders-e2e/04-lifecycle/update lease end date",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/verify lease reminders rescheduled.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/leases/a2428b32-9bbf-4099-9938-2579fe8c65f1/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "b980095b-ff1f-4530-95df-a15ede0296b6",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT",
            "content-length": "1418"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:52.606607+03:00",
                "event_type": "lease_expiring",
                "failed_attempts": 0,
                "id": "7c300a28-afe3-4a12-9575-74ebe63fddb2",
                "lease_id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
                "message_body": "Аренда по объекту заканчивается 02.08.2026",
                "message_title": "Аренда скоро заканчивается",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
                "recurring_operation_id": null,
                "scheduled_at": "2026-07-03T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:52.595486+03:00"
              },
              {
                "created_at": "2026-06-18T12:14:52.606607+03:00",
                "event_type": "lease_requires_action",
                "failed_attempts": 0,
                "id": "72867907-4458-487c-8e2b-44e1af7cac89",
                "lease_id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
                "message_body": "Срок аренды закончился. Подтвердите продление или завершение аренды.",
                "message_title": "Аренда требует действия",
                "next_attempt_at": null,
                "operation_id": null,
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
                "recurring_operation_id": null,
                "scheduled_at": "2026-08-03T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T12:14:52.595486+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/a2428b32-9bbf-4099-9938-2579fe8c65f1/reminders",
          "responseTime": 5,
          "duration": 5,
          "size": 1418
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "pT0jZEPOUksMhQp8inrdk"
          },
          {
            "description": "should contain lease_expiring reminder",
            "status": "pass",
            "uid": "FKQQYhvEeinjRwNQpBy89"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "yfcFIEG1R-57kY8LG5Av-"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.14805475,
        "name": "Verify Lease Reminders Rescheduled",
        "path": "reminders-e2e/04-lifecycle/verify lease reminders rescheduled",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/create property complete.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"name\": \"E2E Complete 1781774092854\",\n  \"type\": \"apartment\",\n  \"address\": \"Complete St 1781774092854\",\n  \"description\": \"E2E reminders complete property\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "65888174-d113-457e-8ff2-d9d3e0419988",
            "date": "Thu, 18 Jun 2026 09:14:52 GMT",
            "content-length": "320"
          },
          "data": {
            "address": "Complete St 1781774092854",
            "created_at": "2026-06-18T12:14:52.859027+03:00",
            "description": "E2E reminders complete property",
            "id": "3db2df61-e216-4b78-a3a1-a67de03fc451",
            "name": "E2E Complete 1781774092854",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:52.859027+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 9,
          "duration": 9,
          "size": 320
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "IytOIbShMWu5UyUaohbfd"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "pMpoJNkv3aMqwnnS2phNx"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.154456958,
        "name": "Create Property Complete",
        "path": "reminders-e2e/04-lifecycle/create property complete",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/create lease complete.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"property_id\": \"3db2df61-e216-4b78-a3a1-a67de03fc451\",\n  \"tenant_contact_id\": \"2cdbdd17-cef9-46f9-9134-e43d0ecee37e\",\n  \"start_date\": \"2026-06-18\",\n  \"end_date\": \"2026-07-18\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 21,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease complete\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "e3df4d33-06d9-47f8-9876-7013d1e56855",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "792"
          },
          "data": {
            "comment": "E2E lease complete",
            "created_at": "2026-06-18T12:14:53.012237+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-18",
            "id": "bbac2cd1-b0b8-4d9c-b4ed-124e70cb4569",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 21,
            "property_id": "3db2df61-e216-4b78-a3a1-a67de03fc451",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:53.012237+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 9,
          "duration": 9,
          "size": 792
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "MqC8aAQ742sGiLebvnuhq"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "XdFmLi-L2pOXmPwGqG3Tt"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.158507125,
        "name": "Create Lease Complete",
        "path": "reminders-e2e/04-lifecycle/create lease complete",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/list recurring operations complete.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/recurring-operations",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "cf4a6f4d-1727-401d-b31c-6dafa577e799",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "490"
          },
          "data": {
            "items": [
              {
                "amount_kopecks": 5000000,
                "category": "rent",
                "comment": null,
                "created_at": "2026-06-18T12:14:53.012237+03:00",
                "end_date": "2026-07-18",
                "id": "a5f7dc25-1f89-4984-a3b3-7cb65909de3c",
                "lease_id": "bbac2cd1-b0b8-4d9c-b4ed-124e70cb4569",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "payment_day": 21,
                "periodicity": "monthly",
                "property_id": "3db2df61-e216-4b78-a3a1-a67de03fc451",
                "start_date": "2026-06-18",
                "status": "active",
                "type": "income",
                "updated_at": "2026-06-18T12:14:53.012237+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/recurring-operations",
          "responseTime": 6,
          "duration": 6,
          "size": 490
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "pHW4q3OwSw8dm2H6gYwJp"
          },
          {
            "description": "should contain recurring operation",
            "status": "pass",
            "uid": "UzXxSC1QGwO3gBPk3PLMU"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.150045042,
        "name": "List Recurring Operations Complete",
        "path": "reminders-e2e/04-lifecycle/list recurring operations complete",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/create recurring reminder complete.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/recurring-operations/a5f7dc25-1f89-4984-a3b3-7cb65909de3c/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-18\"\n}"
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "ed12dc11-e5e4-47af-99d4-3f276afcacb0",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "707"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T12:14:53.325595+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "8bef5aa9-88a0-4750-9bf6-1b10c6501761",
                "lease_id": null,
                "message_body": "rent 50000.00 ₽ запланировано на 18.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "36b0682a-878f-4b64-b6ac-48085b30c91a",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "3db2df61-e216-4b78-a3a1-a67de03fc451",
                "recurring_operation_id": "a5f7dc25-1f89-4984-a3b3-7cb65909de3c",
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T12:14:53.322639+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/recurring-operations/a5f7dc25-1f89-4984-a3b3-7cb65909de3c/reminders",
          "responseTime": 11,
          "duration": 11,
          "size": 707
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "fVriHBbc1cWNp6qjC9Brs"
          },
          {
            "description": "should return reminders",
            "status": "pass",
            "uid": "3-2cD0MjCjtWZnrnoZ2Ob"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.158010542,
        "name": "Create Recurring Reminder Complete",
        "path": "reminders-e2e/04-lifecycle/create recurring reminder complete",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/complete lease.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases/bbac2cd1-b0b8-4d9c-b4ed-124e70cb4569/complete",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "1df29586-8035-4d48-aae5-466312d9c440",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "795"
          },
          "data": {
            "comment": "E2E lease complete",
            "created_at": "2026-06-18T12:14:53.012237+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-18",
            "id": "bbac2cd1-b0b8-4d9c-b4ed-124e70cb4569",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 21,
            "property_id": "3db2df61-e216-4b78-a3a1-a67de03fc451",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:53.478922+03:00"
          },
          "url": "http://localhost/leases/bbac2cd1-b0b8-4d9c-b4ed-124e70cb4569/complete",
          "responseTime": 13,
          "duration": 13,
          "size": 795
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "ai3d6QLcIcMpdcgluuE0I"
          },
          {
            "description": "should have matching id",
            "status": "pass",
            "uid": "LLzF34Jzsb5_2pmurq08B"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.107432958,
        "name": "Complete Lease",
        "path": "reminders-e2e/04-lifecycle/complete lease",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/verify lease reminders complete cancelled.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/leases/bbac2cd1-b0b8-4d9c-b4ed-124e70cb4569/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "3e63427b-9037-414d-86db-754216ebe89f",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "13"
          },
          "data": {
            "items": []
          },
          "url": "http://localhost/leases/bbac2cd1-b0b8-4d9c-b4ed-124e70cb4569/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 13
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "aAr-_uoDO1Q_n_GX3S7qH"
          },
          {
            "description": "should have no lease reminders",
            "status": "pass",
            "uid": "Y4P36a_-fswyZffsL8A5a"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.098883917,
        "name": "Verify Lease Reminders Complete Cancelled",
        "path": "reminders-e2e/04-lifecycle/verify lease reminders complete cancelled",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/04-lifecycle/verify recurring reminders complete cancelled.bru"
        },
        "request": {
          "method": "GET",
          "url": "http://localhost:8080/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/recurring-operations/a5f7dc25-1f89-4984-a3b3-7cb65909de3c/reminders",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "7eabb0a2-5a98-43b3-ba73-be677f0cefec",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "13"
          },
          "data": {
            "items": []
          },
          "url": "http://localhost/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/recurring-operations/a5f7dc25-1f89-4984-a3b3-7cb65909de3c/reminders",
          "responseTime": 3,
          "duration": 3,
          "size": 13
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "vgK4lSGwmgfbcFCWewZvr"
          },
          {
            "description": "should not contain previous recurring reminder",
            "status": "pass",
            "uid": "sFGGOw5YXWpgc6_sUmbvz"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.116778208,
        "name": "Verify Recurring Reminders Complete Cancelled",
        "path": "reminders-e2e/04-lifecycle/verify recurring reminders complete cancelled",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/complete lease 30d.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases/5f37025f-f1bb-472b-b31e-e6c9fb449e17/complete",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "74ef0164-9f0d-4dfd-89df-fc8a35ca5c1b",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "788"
          },
          "data": {
            "comment": "E2E lease 30d",
            "created_at": "2026-06-18T12:14:46.910383+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-18",
            "id": "5f37025f-f1bb-472b-b31e-e6c9fb449e17",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:53.81476+03:00"
          },
          "url": "http://localhost/leases/5f37025f-f1bb-472b-b31e-e6c9fb449e17/complete",
          "responseTime": 14,
          "duration": 14,
          "size": 788
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "f31xnQJbJrhATDW__3-yS"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.12623125,
        "name": "Complete Lease 30d",
        "path": "reminders-e2e/05-cleanup/complete lease 30d",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/complete lease short.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases/63e50dc6-6711-4554-a04f-bd923683da64/complete",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "c31dbb9c-28f8-4ed5-8be4-c3a0891e74f0",
            "date": "Thu, 18 Jun 2026 09:14:53 GMT",
            "content-length": "791"
          },
          "data": {
            "comment": "E2E lease short",
            "created_at": "2026-06-18T12:14:47.253904+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-03",
            "id": "63e50dc6-6711-4554-a04f-bd923683da64",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "a220f042-ac35-4bf8-90b1-2fdd4aaf898a",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:53.929053+03:00"
          },
          "url": "http://localhost/leases/63e50dc6-6711-4554-a04f-bd923683da64/complete",
          "responseTime": 10,
          "duration": 10,
          "size": 791
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "aa6tdS7mnL_egtA8fbtsV"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.112308458,
        "name": "Complete Lease Short",
        "path": "reminders-e2e/05-cleanup/complete lease short",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/complete lease past.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases/ade7779e-53db-486c-af4f-1465a64bc5b1/complete",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "3f1c76ed-5681-4fdb-9450-211c2d44735d",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "790"
          },
          "data": {
            "comment": "E2E lease past",
            "created_at": "2026-06-18T12:14:47.606452+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-06-13",
            "id": "ade7779e-53db-486c-af4f-1465a64bc5b1",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "80f2415d-4637-4053-b393-6e61545d5e45",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-05-19",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:54.042106+03:00"
          },
          "url": "http://localhost/leases/ade7779e-53db-486c-af4f-1465a64bc5b1/complete",
          "responseTime": 10,
          "duration": 10,
          "size": 790
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "8Tm_mEGJJW9wvBkjwEfUi"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.111988,
        "name": "Complete Lease Past",
        "path": "reminders-e2e/05-cleanup/complete lease past",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/complete lease lifecycle.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/leases/a2428b32-9bbf-4099-9938-2579fe8c65f1/complete",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "6b546ea6-1a43-4a99-beba-ce0d9384f8c6",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "802"
          },
          "data": {
            "comment": "E2E lease lifecycle updated",
            "created_at": "2026-06-18T12:14:47.93422+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-08-02",
            "id": "a2428b32-9bbf-4099-9938-2579fe8c65f1",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-18",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T12:14:46.275136+03:00",
              "email": "tenant-1781774086271@example.com",
              "id": "2cdbdd17-cef9-46f9-9134-e43d0ecee37e",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79154086271",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T12:14:46.275136+03:00"
            },
            "updated_at": "2026-06-18T12:14:54.154715+03:00"
          },
          "url": "http://localhost/leases/a2428b32-9bbf-4099-9938-2579fe8c65f1/complete",
          "responseTime": 12,
          "duration": 12,
          "size": 802
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "CG9VphC-YmLnrGywkTKGG"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.110862459,
        "name": "Complete Lease Lifecycle",
        "path": "reminders-e2e/05-cleanup/complete lease lifecycle",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/archive property main.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/archive",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "12b35e97-9f1e-476a-aea1-c749883c1e1b",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "321"
          },
          "data": {
            "address": "Main St 1781774086057",
            "created_at": "2026-06-18T12:14:46.089985+03:00",
            "description": "E2E reminders test property main",
            "id": "47425fff-a3ab-4ad0-acb7-a4bfa543ec19",
            "name": "E2E Reminders Main 1781774086057",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:54.261828+03:00"
          },
          "url": "http://localhost/properties/47425fff-a3ab-4ad0-acb7-a4bfa543ec19/archive",
          "responseTime": 20,
          "duration": 20,
          "size": 321
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "KT8p_ly8ibJQZXTYFAOyT"
          },
          {
            "description": "should be archived",
            "status": "pass",
            "uid": "LjlUFsXIij1ilKJDh3J3B"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.116591917,
        "name": "Archive Property Main",
        "path": "reminders-e2e/05-cleanup/archive property main",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/archive property lease 30d.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/7cbef75c-a65c-46dd-b655-5b135c26a322/archive",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "25896949-bcac-4c74-afdd-796030d27a5c",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "321"
          },
          "data": {
            "address": "Lease 30d St 1781774086749",
            "created_at": "2026-06-18T12:14:46.753056+03:00",
            "description": "E2E reminders lease 30d property",
            "id": "7cbef75c-a65c-46dd-b655-5b135c26a322",
            "name": "E2E Lease 30d 1781774086749",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:54.378729+03:00"
          },
          "url": "http://localhost/properties/7cbef75c-a65c-46dd-b655-5b135c26a322/archive",
          "responseTime": 9,
          "duration": 9,
          "size": 321
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "-G3tSgN3RDEc2_0nOVg-L"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.106079583,
        "name": "Archive Property Lease 30d",
        "path": "reminders-e2e/05-cleanup/archive property lease 30d",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/archive property lease short.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/a220f042-ac35-4bf8-90b1-2fdd4aaf898a/archive",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "1f848246-abcf-421f-bc13-c7bee479150b",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "327"
          },
          "data": {
            "address": "Lease Short St 1781774087075",
            "created_at": "2026-06-18T12:14:47.078443+03:00",
            "description": "E2E reminders lease short property",
            "id": "a220f042-ac35-4bf8-90b1-2fdd4aaf898a",
            "name": "E2E Lease Short 1781774087075",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:54.485017+03:00"
          },
          "url": "http://localhost/properties/a220f042-ac35-4bf8-90b1-2fdd4aaf898a/archive",
          "responseTime": 8,
          "duration": 8,
          "size": 327
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "zdUgLhHd1NKQMQVdYgRqf"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.104144958,
        "name": "Archive Property Lease Short",
        "path": "reminders-e2e/05-cleanup/archive property lease short",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/archive property lease past.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/80f2415d-4637-4053-b393-6e61545d5e45/archive",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "e94af1be-b408-430c-bde0-75de1f391970",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "324"
          },
          "data": {
            "address": "Lease Past St 1781774087432",
            "created_at": "2026-06-18T12:14:47.436203+03:00",
            "description": "E2E reminders lease past property",
            "id": "80f2415d-4637-4053-b393-6e61545d5e45",
            "name": "E2E Lease Past 1781774087432",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:54.589205+03:00"
          },
          "url": "http://localhost/properties/80f2415d-4637-4053-b393-6e61545d5e45/archive",
          "responseTime": 8,
          "duration": 8,
          "size": 324
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "phNR2hFW5ySNrwupN1bwi"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.104776625,
        "name": "Archive Property Lease Past",
        "path": "reminders-e2e/05-cleanup/archive property lease past",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/archive property lifecycle.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/bfcb66ed-ec8c-43d3-b512-bd915f69cc90/archive",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "1df91b46-32c1-47f2-a942-558872c89bae",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "321"
          },
          "data": {
            "address": "Lifecycle St 1781774087772",
            "created_at": "2026-06-18T12:14:47.774921+03:00",
            "description": "E2E reminders lifecycle property",
            "id": "bfcb66ed-ec8c-43d3-b512-bd915f69cc90",
            "name": "E2E Lifecycle 1781774087772",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:54.694574+03:00"
          },
          "url": "http://localhost/properties/bfcb66ed-ec8c-43d3-b512-bd915f69cc90/archive",
          "responseTime": 9,
          "duration": 9,
          "size": 321
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "9ucYhYZKtQoPQTsxGXj3Q"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.104029125,
        "name": "Archive Property Lifecycle",
        "path": "reminders-e2e/05-cleanup/archive property lifecycle",
        "iterationIndex": 0
      },
      {
        "test": {
          "filename": "reminders-e2e/05-cleanup/archive property complete.bru"
        },
        "request": {
          "method": "POST",
          "url": "http://localhost:8080/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/archive",
          "headers": {
            "content-type": null,
            "Cookie": "********"
          }
        },
        "response": {
          "status": 200,
          "statusText": "OK",
          "headers": {
            "content-security-policy": "default-src 'none'",
            "content-type": "application/json",
            "referrer-policy": "strict-origin-when-cross-origin",
            "x-content-type-options": "nosniff",
            "x-frame-options": "DENY",
            "x-request-id": "cefbc27e-d08c-4cad-97ad-49b481abf055",
            "date": "Thu, 18 Jun 2026 09:14:54 GMT",
            "content-length": "317"
          },
          "data": {
            "address": "Complete St 1781774092854",
            "created_at": "2026-06-18T12:14:52.859027+03:00",
            "description": "E2E reminders complete property",
            "id": "3db2df61-e216-4b78-a3a1-a67de03fc451",
            "name": "E2E Complete 1781774092854",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T12:14:54.79786+03:00"
          },
          "url": "http://localhost/properties/3db2df61-e216-4b78-a3a1-a67de03fc451/archive",
          "responseTime": 11,
          "duration": 11,
          "size": 317
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "ttWsx_eqg7eE7pFiLOW1E"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.107233959,
        "name": "Archive Property Complete",
        "path": "reminders-e2e/05-cleanup/archive property complete",
        "iterationIndex": 0
      }
    ],
    "summary": {
      "totalRequests": 68,
      "passedRequests": 68,
      "failedRequests": 0,
      "errorRequests": 0,
      "skippedRequests": 0,
      "totalAssertions": 0,
      "passedAssertions": 0,
      "failedAssertions": 0,
      "totalTests": 130,
      "passedTests": 130,
      "failedTests": 0,
      "totalPreRequestTests": 0,
      "passedPreRequestTests": 0,
      "failedPreRequestTests": 0,
      "totalPostResponseTests": 0,
      "passedPostResponseTests": 0,
      "failedPostResponseTests": 0
    }
  }
]
```

## DB Verification

```
          metric           |  status   | cnt 
---------------------------+-----------+-----
 cancelled_reminders       |           |  28
 failed_reminders          |           |   0
 sent_sms_audit_rows       |           |   1
 total_reminders_by_status | pending   |   3
 total_reminders_by_status | sent      |   1
 total_reminders_by_status | cancelled |  28
(6 rows)
```

## Recent Backend Log Excerpt

```
[2m12:14:48[0m [92mINF[0m request handled [2mrequest_id=[0m3a84287c-96dd-4c9e-a93c-168f710467a7 [2mmethod=[0mGET [2mroute=[0m/leases/{leaseId}/reminders [2mstatus=[0m200 [2mduration=[0m3.732709ms
[2m12:14:49[0m [92mINF[0m request handled [2mrequest_id=[0mec481613-c565-4dba-b8df-e18a650d6d48 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m201 [2mduration=[0m5.940583ms
[2m12:14:49[0m [92mINF[0m request handled [2mrequest_id=[0maa704bfb-4ce8-4435-a8d1-8ddea1ec7dda [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m200 [2mduration=[0m4.128041ms
[2m12:14:49[0m [92mINF[0m request handled [2mrequest_id=[0m33988e58-2e23-4924-af30-a7b2b4ae325c [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m21.738125ms
[2m12:14:49[0m [92mINF[0m request handled [2mrequest_id=[0m5c5f9340-cbec-4b8a-a211-d3bf851332df [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m200 [2mduration=[0m2.685667ms
[2m12:14:49[0m [92mINF[0m request handled [2mrequest_id=[0mf1340b48-5e8f-490a-b4c5-1d396073447e [2mmethod=[0mGET [2mroute=[0m/leases/{leaseId}/reminders [2mstatus=[0m200 [2mduration=[0m3.78775ms
[2m12:14:49[0m [92mINF[0m request handled [2mrequest_id=[0md745a22f-1826-4cd4-94f9-91ce134d8229 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m4.209917ms
[2m12:14:49[0m [92mINF[0m request handled [2mrequest_id=[0m3f226439-97f1-4a7c-b01a-450b491ca8ad [2mmethod=[0mPATCH [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m200 [2mduration=[0m5.283875ms
[2m12:14:49[0m [93mWRN[0m request handled [2mrequest_id=[0m4117a124-4b9f-4be0-8824-7dfca49e4b5a [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m401 [2mduration=[0m20.792µs [2merror_code=[0mUnauthorized
[2m12:14:50[0m [93mWRN[0m request handled [2mrequest_id=[0m52ede516-edaa-465a-b482-5e1070fdf2a8 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m404 [2mduration=[0m853.792µs [2merror_code=[0m"Not found"
[2m12:14:50[0m [93mWRN[0m request handled [2mrequest_id=[0m4d39b06e-8266-44da-a3c5-a24f4a740a27 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m404 [2mduration=[0m3.015958ms [2merror_code=[0m"Not found"
[2m12:14:50[0m [93mWRN[0m request handled [2mrequest_id=[0mfecc89b9-f3dd-4fd3-be8a-09d824171cea [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m400 [2mduration=[0m4.49175ms [2merror_code=[0m"Bad request"
[2m12:14:50[0m [93mWRN[0m request handled [2mrequest_id=[0m2ff58a61-c1ad-4a60-977b-52e0602f5b9c [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m400 [2mduration=[0m5.06125ms [2merror_code=[0m"Bad request"
[2m12:14:50[0m [92mINF[0m request handled [2mrequest_id=[0md908bd92-cf51-4ce7-80de-2077e89bedcb [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m201 [2mduration=[0m5.698417ms
[2m12:14:50[0m [92mINF[0m request handled [2mrequest_id=[0m0ddf4ddd-d6f9-4901-bfcf-7f07b7293d37 [2mmethod=[0mDELETE [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m204 [2mduration=[0m3.630875ms
[2m12:14:50[0m [93mWRN[0m request handled [2mrequest_id=[0m6d1f4205-bfcf-42cb-8b1a-b72b60b96cd8 [2mmethod=[0mPATCH [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m400 [2mduration=[0m1.687625ms [2merror_code=[0m"Bad request"
[2m12:14:50[0m [93mWRN[0m request handled [2mrequest_id=[0m64257e5a-72d8-4b73-8e7c-9d760cff3bc8 [2mmethod=[0mPATCH [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m404 [2mduration=[0m2.32425ms [2merror_code=[0m"Not found"
[2m12:14:50[0m [93mWRN[0m request handled [2mrequest_id=[0m37d1f485-e91f-4513-9eca-f27a88e7744f [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m400 [2mduration=[0m1.660333ms [2merror_code=[0m"Bad request"
[2m12:14:51[0m [92mINF[0m request handled [2mrequest_id=[0maea39e01-42c9-435e-800a-eb32878eee1c [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m201 [2mduration=[0m5.283167ms
[2m12:14:51[0m [92mINF[0m request handled [2mrequest_id=[0m15243804-5d2f-4059-b624-698d1028fa60 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m16.006458ms
[2m12:14:51[0m [93mWRN[0m request handled [2mrequest_id=[0m4b257a6b-e891-4564-88ed-7e83a8d356b3 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m400 [2mduration=[0m4.549875ms [2merror_code=[0m"Bad request"
[2m12:14:51[0m [92mINF[0m request handled [2mrequest_id=[0m72346d2d-7e46-46b9-b7db-ac1b1842931f [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m3.033708ms
[2m12:14:51[0m [92mINF[0m request handled [2mrequest_id=[0m554628e8-1777-49a3-ad7a-6bcb8a064ce1 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m3.271875ms
[2m12:14:51[0m [92mINF[0m request handled [2mrequest_id=[0m9d4a6fd1-1b16-4ea6-8365-29d983371694 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m1.732917ms
[2m12:14:51[0m [92mINF[0m request handled [2mrequest_id=[0m735749a1-be36-47de-8eda-fb26af27b43a [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m16.442ms
[2m12:14:51[0m [92mINF[0m request handled [2mrequest_id=[0m731582b5-c0eb-4e33-85ee-76304ee50d2e [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m16.080083ms
[2m12:14:52[0m [92mINF[0m request handled [2mrequest_id=[0md6369e34-eeb5-4b1e-a87b-efec7cdb3ea4 [2mmethod=[0mPATCH [2mroute=[0m/operations/{id} [2mstatus=[0m200 [2mduration=[0m6.295ms
[2m12:14:52[0m [92mINF[0m request handled [2mrequest_id=[0m45cf3db6-05b4-45a2-aa41-b12b1548fed1 [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m200 [2mduration=[0m2.625084ms
[2m12:14:52[0m [92mINF[0m request handled [2mrequest_id=[0m27d130f7-71c9-48ed-af7f-9f1b26de049a [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m201 [2mduration=[0m4.325166ms
[2m12:14:52[0m [92mINF[0m request handled [2mrequest_id=[0mb983f2ef-b519-4513-9eeb-5da14505d6c7 [2mmethod=[0mDELETE [2mroute=[0m/operations/{id} [2mstatus=[0m204 [2mduration=[0m6.641792ms
[2m12:14:52[0m [93mWRN[0m request handled [2mrequest_id=[0ma18e3bac-cc73-4c1f-97ba-9cd7c29d453e [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m404 [2mduration=[0m2.455375ms [2merror_code=[0m"Not found"
[2m12:14:52[0m [92mINF[0m request handled [2mrequest_id=[0m6141bad3-e8da-4796-9a08-17ed514d5c75 [2mmethod=[0mPATCH [2mroute=[0m/leases/{id} [2mstatus=[0m200 [2mduration=[0m15.943208ms
[2m12:14:52[0m [92mINF[0m request handled [2mrequest_id=[0mb980095b-ff1f-4530-95df-a15ede0296b6 [2mmethod=[0mGET [2mroute=[0m/leases/{leaseId}/reminders [2mstatus=[0m200 [2mduration=[0m4.001917ms
[2m12:14:52[0m [92mINF[0m request handled [2mrequest_id=[0m65888174-d113-457e-8ff2-d9d3e0419988 [2mmethod=[0mPOST [2mroute=[0m/properties [2mstatus=[0m201 [2mduration=[0m7.180458ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0me3df4d33-06d9-47f8-9876-7013d1e56855 [2mmethod=[0mPOST [2mroute=[0m/leases [2mstatus=[0m201 [2mduration=[0m8.528125ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0mcf4a6f4d-1727-401d-b31c-6dafa577e799 [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/recurring-operations [2mstatus=[0m200 [2mduration=[0m5.36425ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0med12dc11-e5e4-47af-99d4-3f276afcacb0 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m10.371542ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0m1df29586-8035-4d48-aae5-466312d9c440 [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m11.118459ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0m3e63427b-9037-414d-86db-754216ebe89f [2mmethod=[0mGET [2mroute=[0m/leases/{leaseId}/reminders [2mstatus=[0m200 [2mduration=[0m2.814625ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0m7eabb0a2-5a98-43b3-ba73-be677f0cefec [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m200 [2mduration=[0m2.122833ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0m74ef0164-9f0d-4dfd-89df-fc8a35ca5c1b [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m12.378833ms
[2m12:14:53[0m [92mINF[0m request handled [2mrequest_id=[0mc31dbb9c-28f8-4ed5-8be4-c3a0891e74f0 [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m9.118833ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0m3f1c76ed-5681-4fdb-9450-211c2d44735d [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m8.776875ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0m6b546ea6-1a43-4a99-beba-ce0d9384f8c6 [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m11.128875ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0m12b35e97-9f1e-476a-aea1-c749883c1e1b [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m18.80375ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0m25896949-bcac-4c74-afdd-796030d27a5c [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m8.682875ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0m1f848246-abcf-421f-bc13-c7bee479150b [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m6.75475ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0me94af1be-b408-430c-bde0-75de1f391970 [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m6.965916ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0m1df91b46-32c1-47f2-a942-558872c89bae [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m7.396083ms
[2m12:14:54[0m [92mINF[0m request handled [2mrequest_id=[0mcefbc27e-d08c-4cad-97ad-49b481abf055 [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m10.167542ms
```
