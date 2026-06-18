# Reminders API E2E Test Report

- **Date:** 2026-06-17T23:45:47Z
- **Commit:** a7bfcc3a253e421478f00d9644b61cabf770f880
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
          "data": "{\n  \"name\": \"E2E Reminders Main 1781739900516\",\n  \"type\": \"apartment\",\n  \"address\": \"Main St 1781739900516\",\n  \"description\": \"E2E reminders test property main\"\n}"
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
            "x-request-id": "196e5f88-ac35-41a6-b33e-69ebaa8a3a0b",
            "date": "Wed, 17 Jun 2026 23:45:00 GMT",
            "content-length": "323"
          },
          "data": {
            "address": "Main St 1781739900516",
            "created_at": "2026-06-18T02:45:00.549605+03:00",
            "description": "E2E reminders test property main",
            "id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "name": "E2E Reminders Main 1781739900516",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:00.549605+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 20,
          "duration": 20,
          "size": 323
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "knplqJe_OhvPPv3yu7j-m"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "7yKlBdFo4Toz7BAQbBuf7"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.233618792,
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
          "data": "{\n  \"name\": \"Ivan\",\n  \"surname\": \"Ivanov\",\n  \"patronymic\": \"Ivanovich\",\n  \"phone\": \"+79159900731\",\n  \"email\": \"tenant-1781739900731@example.com\",\n  \"comment\": \"Primary tenant contact\"\n}"
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
            "x-request-id": "311876dd-388e-4d81-9ef6-75fed7daf887",
            "date": "Wed, 17 Jun 2026 23:45:00 GMT",
            "content-length": "351"
          },
          "data": {
            "comment": "Primary tenant contact",
            "created_at": "2026-06-18T02:45:00.735538+03:00",
            "email": "tenant-1781739900731@example.com",
            "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
            "name": "Ivan",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "patronymic": "Ivanovich",
            "phone": "+79159900731",
            "surname": "Ivanov",
            "updated_at": "2026-06-18T02:45:00.735538+03:00"
          },
          "url": "http://localhost/tenant-contacts",
          "responseTime": 6,
          "duration": 6,
          "size": 351
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "L8D_7bYarm8h7vtm0_48N"
          },
          {
            "description": "should have tenant contact id",
            "status": "pass",
            "uid": "QW_gtaqrqLB-_1jV27fTV"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.183584041,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"income\",\n  \"category\": \"rent\",\n  \"amount_kopecks\": 150000,\n  \"operation_date\": \"2026-06-24\",\n  \"comment\": \"E2E manual operation main\"\n}"
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
            "x-request-id": "86fe6cef-7e1a-445e-a3ff-868b20d5f95b",
            "date": "Wed, 17 Jun 2026 23:45:00 GMT",
            "content-length": "437"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E manual operation main",
            "created_at": "2026-06-18T02:45:00.920389+03:00",
            "id": "9b53d867-6cd6-440e-b63a-02a8c63a53db",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-24",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T02:45:00.920389+03:00"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations",
          "responseTime": 9,
          "duration": 9,
          "size": 437
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "8u-IfoNZLfbW2QyEyOwd1"
          },
          {
            "description": "should have operation id",
            "status": "pass",
            "uid": "XmsAB1Jv5H8r3apeKNqka"
          },
          {
            "description": "should match operation date",
            "status": "pass",
            "uid": "1Y219PKv5XOFZW4rAatUJ"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.184347583,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"expense\",\n  \"category\": \"utilities\",\n  \"amount_kopecks\": 50000,\n  \"start_date\": \"2026-06-20\",\n  \"payment_day\": 20,\n  \"comment\": \"E2E recurring operation main\"\n}"
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
            "x-request-id": "a4e47c3a-1ba5-4f30-8ece-6481738e1b2f",
            "date": "Wed, 17 Jun 2026 23:45:01 GMT",
            "content-length": "466"
          },
          "data": {
            "amount_kopecks": 50000,
            "category": "utilities",
            "comment": "E2E recurring operation main",
            "created_at": "2026-06-18T02:45:01.102196+03:00",
            "end_date": null,
            "id": "e26c22d7-9044-421e-901d-811b6f451058",
            "lease_id": null,
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 20,
            "periodicity": "monthly",
            "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "start_date": "2026-06-20",
            "status": "active",
            "type": "expense",
            "updated_at": "2026-06-18T02:45:01.102196+03:00"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations",
          "responseTime": 14,
          "duration": 14,
          "size": 466
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "Pzyj5QrCC3z-SlDfFluf7"
          },
          {
            "description": "should have recurring operation id",
            "status": "pass",
            "uid": "HuoqwSfapLwp0wX2GBAdj"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.189165875,
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
          "data": "{\n  \"name\": \"E2E Lease 30d 1781739901287\",\n  \"type\": \"apartment\",\n  \"address\": \"Lease 30d St 1781739901287\",\n  \"description\": \"E2E reminders lease 30d property\"\n}"
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
            "x-request-id": "ef2c0c8b-b5d8-4bd8-a811-10b5b1b6132a",
            "date": "Wed, 17 Jun 2026 23:45:01 GMT",
            "content-length": "323"
          },
          "data": {
            "address": "Lease 30d St 1781739901287",
            "created_at": "2026-06-18T02:45:01.290568+03:00",
            "description": "E2E reminders lease 30d property",
            "id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
            "name": "E2E Lease 30d 1781739901287",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:01.290568+03:00"
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
            "uid": "JXjNzNYnPr0XfNmJN8b2E"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "FtMnH4H7ikQbucyv0MV5K"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.182217,
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
          "data": "{\n  \"property_id\": \"f978f1c6-c69c-43f6-bc9f-b3e765d05089\",\n  \"tenant_contact_id\": \"9d1bb26e-d63c-4606-ba90-9e7c7ab359f7\",\n  \"start_date\": \"2026-06-17\",\n  \"end_date\": \"2026-07-17\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease 30d\"\n}"
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
            "x-request-id": "0c2aede3-31bf-4f6e-8d7c-4a153f8d0756",
            "date": "Wed, 17 Jun 2026 23:45:01 GMT",
            "content-length": "786"
          },
          "data": {
            "comment": "E2E lease 30d",
            "created_at": "2026-06-18T02:45:01.477173+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-17",
            "id": "f8f3df33-2fa4-4aa4-ad97-7372eb9d73df",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:01.477173+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 15,
          "duration": 15,
          "size": 786
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "qEQBJuVXz3PxOv39oIZ-E"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "rxXb1smOQYSF7K7gtk1cf"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.190595417,
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
          "data": "{\n  \"name\": \"E2E Lease Short 1781739901660\",\n  \"type\": \"apartment\",\n  \"address\": \"Lease Short St 1781739901660\",\n  \"description\": \"E2E reminders lease short property\"\n}"
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
            "x-request-id": "69e27e6e-14ca-436c-84b0-78505f1130ed",
            "date": "Wed, 17 Jun 2026 23:45:01 GMT",
            "content-length": "327"
          },
          "data": {
            "address": "Lease Short St 1781739901660",
            "created_at": "2026-06-18T02:45:01.66242+03:00",
            "description": "E2E reminders lease short property",
            "id": "44bc828a-fd1e-410c-95c7-73fc77034005",
            "name": "E2E Lease Short 1781739901660",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:01.66242+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 6,
          "duration": 6,
          "size": 327
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "-TeJl8hB7SxeX8GCT2ntZ"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "StxH7nvyCBUXquBlvwjqF"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.179920709,
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
          "data": "{\n  \"property_id\": \"44bc828a-fd1e-410c-95c7-73fc77034005\",\n  \"tenant_contact_id\": \"9d1bb26e-d63c-4606-ba90-9e7c7ab359f7\",\n  \"start_date\": \"2026-06-17\",\n  \"end_date\": \"2026-07-02\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease short\"\n}"
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
            "x-request-id": "7776a9d9-6338-4bd3-ac12-77a499e6353a",
            "date": "Wed, 17 Jun 2026 23:45:01 GMT",
            "content-length": "788"
          },
          "data": {
            "comment": "E2E lease short",
            "created_at": "2026-06-18T02:45:01.846591+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-02",
            "id": "e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "44bc828a-fd1e-410c-95c7-73fc77034005",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:01.846591+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 14,
          "duration": 14,
          "size": 788
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "VFcQvdti31RUrQoR1nEBO"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "yr3xOa8n8uhTnHUiXr8pG"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.193355375,
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
          "data": "{\n  \"name\": \"E2E Lease Past 1781739902033\",\n  \"type\": \"apartment\",\n  \"address\": \"Lease Past St 1781739902033\",\n  \"description\": \"E2E reminders lease past property\"\n}"
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
            "x-request-id": "df62fc8d-dd1f-4f48-b1d2-575c10ef5b73",
            "date": "Wed, 17 Jun 2026 23:45:02 GMT",
            "content-length": "326"
          },
          "data": {
            "address": "Lease Past St 1781739902033",
            "created_at": "2026-06-18T02:45:02.035985+03:00",
            "description": "E2E reminders lease past property",
            "id": "3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96",
            "name": "E2E Lease Past 1781739902033",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:02.035985+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 6,
          "duration": 6,
          "size": 326
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "vhPoHUmt2XmGpa3fPrZHN"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "SJPilTxovoVG8LnCWLoDG"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.183997291,
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
          "data": "{\n  \"property_id\": \"3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96\",\n  \"tenant_contact_id\": \"9d1bb26e-d63c-4606-ba90-9e7c7ab359f7\",\n  \"start_date\": \"2026-05-18\",\n  \"end_date\": \"2026-06-12\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease past\"\n}"
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
            "x-request-id": "b9cbe346-d346-4b90-8717-cd46461ded2c",
            "date": "Wed, 17 Jun 2026 23:45:02 GMT",
            "content-length": "796"
          },
          "data": {
            "comment": "E2E lease past",
            "created_at": "2026-06-18T02:45:02.223035+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-06-12",
            "id": "e873f094-d8eb-4736-a45c-586e9ef7833f",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-05-18",
            "status": "requires_action",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:02.223035+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 9,
          "duration": 9,
          "size": 796
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "M69R8glCNYZxrN9RzxsXd"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "KBSos1RTqK7HJmFZcqvMR"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.191597166,
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
          "data": "{\n  \"name\": \"E2E Lifecycle 1781739902409\",\n  \"type\": \"apartment\",\n  \"address\": \"Lifecycle St 1781739902409\",\n  \"description\": \"E2E reminders lifecycle property\"\n}"
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
            "x-request-id": "50a0ac7d-7aeb-42c2-a706-31dd1381d733",
            "date": "Wed, 17 Jun 2026 23:45:02 GMT",
            "content-length": "323"
          },
          "data": {
            "address": "Lifecycle St 1781739902409",
            "created_at": "2026-06-18T02:45:02.412691+03:00",
            "description": "E2E reminders lifecycle property",
            "id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
            "name": "E2E Lifecycle 1781739902409",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:02.412691+03:00"
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
            "uid": "kbfXanSZD0PPF0Cv4b7GU"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "Hg7wgA70_CvOjY8AwbFti"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.184516958,
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
          "data": "{\n  \"property_id\": \"bf10cc7e-c23b-4fbb-945e-4047b5b6cafa\",\n  \"tenant_contact_id\": \"9d1bb26e-d63c-4606-ba90-9e7c7ab359f7\",\n  \"start_date\": \"2026-06-17\",\n  \"end_date\": \"2026-07-17\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 1,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease lifecycle\"\n}"
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
            "x-request-id": "7cb1525e-8288-4325-afd0-71c1eb83ffdf",
            "date": "Wed, 17 Jun 2026 23:45:02 GMT",
            "content-length": "792"
          },
          "data": {
            "comment": "E2E lease lifecycle",
            "created_at": "2026-06-18T02:45:02.599338+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-17",
            "id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:02.599338+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 10,
          "duration": 10,
          "size": 792
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "DrfbifZ3vRJHUIHdo3GI3"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "XN9jYVsIf5aJv5cJURUGr"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.188893083,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"income\",\n  \"category\": \"rent\",\n  \"amount_kopecks\": 150000,\n  \"operation_date\": \"2026-06-17\",\n  \"comment\": \"E2E operation due today\"\n}"
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
            "x-request-id": "a194d6a5-adb9-46f6-9e9b-57f84cfdc149",
            "date": "Wed, 17 Jun 2026 23:45:02 GMT",
            "content-length": "433"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E operation due today",
            "created_at": "2026-06-18T02:45:02.78898+03:00",
            "id": "36495d17-732b-4016-8d2c-4eecdbd46bf1",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-17",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T02:45:02.78898+03:00"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations",
          "responseTime": 3,
          "duration": 3,
          "size": 433
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "SpTnLGBMKRIIfs2HFvlEv"
          },
          {
            "description": "should have operation id",
            "status": "pass",
            "uid": "queFqYu6Kcav2ZMMLwhgh"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.18415875,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"income\",\n  \"category\": \"rent\",\n  \"amount_kopecks\": 150000,\n  \"operation_date\": \"2026-06-24\",\n  \"comment\": \"E2E operation to delete\"\n}"
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
            "x-request-id": "947bec2c-97b5-4e8b-9448-1fa750c08c35",
            "date": "Wed, 17 Jun 2026 23:45:02 GMT",
            "content-length": "435"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E operation to delete",
            "created_at": "2026-06-18T02:45:02.972661+03:00",
            "id": "e345ebe2-3958-4d12-8ec0-47ca8064927c",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-24",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T02:45:02.972661+03:00"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations",
          "responseTime": 5,
          "duration": 5,
          "size": 435
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "qh4bGKIkvolpQksX0sTBQ"
          },
          {
            "description": "should have operation id",
            "status": "pass",
            "uid": "ReWkjSfF87QAu_-nPiy9f"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.184568875,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"expense\",\n  \"category\": \"utilities\",\n  \"amount_kopecks\": 50000,\n  \"start_date\": \"2026-05-18\",\n  \"end_date\": \"2026-06-07\",\n  \"payment_day\": 5,\n  \"comment\": \"E2E recurring operation no future\"\n}"
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
            "x-request-id": "15b0ee15-0a59-4444-aa01-ad075d810b44",
            "date": "Wed, 17 Jun 2026 23:45:03 GMT",
            "content-length": "476"
          },
          "data": {
            "amount_kopecks": 50000,
            "category": "utilities",
            "comment": "E2E recurring operation no future",
            "created_at": "2026-06-18T02:45:03.15729+03:00",
            "end_date": "2026-06-07",
            "id": "f3ec10d9-f054-4b1c-8387-93ef2d196131",
            "lease_id": null,
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 5,
            "periodicity": "monthly",
            "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "start_date": "2026-05-18",
            "status": "active",
            "type": "expense",
            "updated_at": "2026-06-18T02:45:03.15729+03:00"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations",
          "responseTime": 13,
          "duration": 13,
          "size": 476
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "57aLs8UsudVgwB4p33oQL"
          },
          {
            "description": "should have recurring operation id",
            "status": "pass",
            "uid": "E3i5XgyQZ6yPf7VBff-nQ"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.197004959,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"type\": \"expense\",\n  \"category\": \"utilities\",\n  \"amount_kopecks\": 50000,\n  \"start_date\": \"2026-06-20\",\n  \"payment_day\": 20,\n  \"comment\": \"E2E recurring operation offset test\"\n}"
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
            "x-request-id": "e7463d25-eebf-4576-9157-c5729f9ace05",
            "date": "Wed, 17 Jun 2026 23:45:03 GMT",
            "content-length": "471"
          },
          "data": {
            "amount_kopecks": 50000,
            "category": "utilities",
            "comment": "E2E recurring operation offset test",
            "created_at": "2026-06-18T02:45:03.35287+03:00",
            "end_date": null,
            "id": "2475d791-8e59-48aa-a991-46da23d13bf7",
            "lease_id": null,
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 20,
            "periodicity": "monthly",
            "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "start_date": "2026-06-20",
            "status": "active",
            "type": "expense",
            "updated_at": "2026-06-18T02:45:03.35287+03:00"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations",
          "responseTime": 16,
          "duration": 16,
          "size": 471
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "GiDQ-PlBfgJOZ9zxlbrX9"
          },
          {
            "description": "should have recurring operation id",
            "status": "pass",
            "uid": "EjWtlojBBFqrtlbgL8Zku"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.191683875,
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
          "url": "http://localhost:8080/leases/f8f3df33-2fa4-4aa4-ad97-7372eb9d73df/reminders",
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
            "x-request-id": "076e8e83-04db-4e84-bb59-bf62c8d4feed",
            "date": "Wed, 17 Jun 2026 23:45:03 GMT",
            "content-length": "1418"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/f8f3df33-2fa4-4aa4-ad97-7372eb9d73df/reminders",
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
            "uid": "L_OYYyCMa2nclAmB1p0KA"
          },
          {
            "description": "should contain lease_expiring reminder",
            "status": "pass",
            "uid": "d0pG6RjBorQ5oazzPkXvd"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "bRTz5z7Y2pr8EaWe4Ib3W"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.193164,
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
          "url": "http://localhost:8080/leases/e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489/reminders",
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
            "x-request-id": "0e3b6577-df6c-495a-9486-7fe937fe2196",
            "date": "Wed, 17 Jun 2026 23:45:03 GMT",
            "content-length": "744"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.846591+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489/reminders",
          "responseTime": 2,
          "duration": 2,
          "size": 744
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "IXYMKKsZ37S5WFxGvgEfw"
          },
          {
            "description": "should not contain lease_expiring reminder",
            "status": "pass",
            "uid": "dLdr12AfWYCJs2QZ9mz7S"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "oq6TeujtPTMb2XPmmEdC8"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.118258792,
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
          "url": "http://localhost:8080/leases/e873f094-d8eb-4736-a45c-586e9ef7833f/reminders",
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
            "x-request-id": "f24d66b2-515f-49f7-a088-514a4c325f11",
            "date": "Wed, 17 Jun 2026 23:45:03 GMT",
            "content-length": "744"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:02.223035+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/e873f094-d8eb-4736-a45c-586e9ef7833f/reminders",
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
            "uid": "f-7UymI7yI6kJD37x1OxO"
          },
          {
            "description": "should not contain lease_expiring reminder",
            "status": "pass",
            "uid": "UnheLo4X6-MBFHwpOcfgg"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "uVkLu8Tix-uslBJvZxATk"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.182280583,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-22\"\n}"
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
            "x-request-id": "83b28192-8a21-40df-841b-8a39864f5e02",
            "date": "Wed, 17 Jun 2026 23:45:04 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-17T23:45:04.038046Z",
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
            "scheduled_at": "2026-06-22T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-17T23:45:04.038046Z"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db/reminders",
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
            "uid": "MsLa3XcXrO8_yd1fzXPZj"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "EP_kx6MxuNsGcczQIfF9i"
          },
          {
            "description": "should be pending",
            "status": "pass",
            "uid": "UDm8TR662YLtZ0bQS5mHE"
          },
          {
            "description": "should be operation target",
            "status": "pass",
            "uid": "6xK0oyNa2Xy5xb8aNvBvs"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.177363541,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db/reminders",
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
            "x-request-id": "7c389c21-9c2e-42f5-a7ca-cd579b8788b3",
            "date": "Wed, 17 Jun 2026 23:45:04 GMT",
            "content-length": "672"
          },
          "data": {
            "items": [
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
                "scheduled_at": "2026-06-22T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.038276+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 672
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "yWwvPBhYwRKwJOm7Jkiuq"
          },
          {
            "description": "should contain created reminder",
            "status": "pass",
            "uid": "06pI-8WIBTn0krRDMQ1PV"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.12045025,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/e26c22d7-9044-421e-901d-811b6f451058/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-20\"\n}"
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
            "x-request-id": "2dbebfa3-abeb-4377-ba8e-74226a244dd0",
            "date": "Wed, 17 Jun 2026 23:45:04 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bbcd4d40-525f-413c-85ef-29022ddec372",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22286c02-b986-4273-a77b-74dac1fc92ea",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-06-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1a390012-c886-4d2e-ac6b-9e36a86677b2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "890dbe88-153c-476e-877e-bcf84643b099",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-07-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "377ddb78-c87c-4d1f-a237-26328ca7d608",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "1422d14d-d0c1-4cdf-b64e-e20111b05d29",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-08-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bfff029c-d810-421b-9981-cbf978c27d00",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da06abee-1dfd-4714-8bda-1e841c25520b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-09-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "294eac1a-4bbe-4704-a9fc-9dba35067d29",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b71c06b1-378e-49e4-8182-707db81cb349",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-10-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "494878f0-fed3-4c03-9f81-739a2373f0a0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3d17a41f-f381-4118-8a9f-d3e355afa9f2",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-11-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "493a29c0-0489-43ee-b899-8d600fedc13b",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35584414-69a9-48eb-b2ff-058deaf81221",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-12-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "5d1e7a80-810a-4352-93da-8bd351356931",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd5376a6-394c-4f40-849c-f6e2ec8d67f7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-01-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "abc2d86a-7cae-4d57-ab23-922f919f6270",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "70845d12-71f7-446a-91aa-ad239610161b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-02-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "56ac1ddf-2e88-41aa-9ba3-dee1fcab07b9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5891fda-6435-465f-8d38-13029b58cc1c",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-03-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2593880d-08db-467b-9c0e-6f69514ea8d3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "39ea5154-7f7f-4562-b0b1-8d0ff137faf0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-04-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ef71066e-161f-4123-b271-b1bcded032f0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "162967ad-700e-49a2-b424-dec2d5756466",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-05-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/e26c22d7-9044-421e-901d-811b6f451058/reminders",
          "responseTime": 19,
          "duration": 19,
          "size": 8376
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "7gTUjjnjU-VlEHkAKBoY4"
          },
          {
            "description": "should return array of reminders",
            "status": "pass",
            "uid": "PAepDYPgFdCrdQDAWnD7n"
          },
          {
            "description": "should have reminder ids",
            "status": "pass",
            "uid": "tQ9MVIX_T8e28JL2gN7Gj"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.195157959,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/e26c22d7-9044-421e-901d-811b6f451058/reminders",
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
            "x-request-id": "614d6a0b-2b30-42dc-a45e-afb569047a14",
            "date": "Wed, 17 Jun 2026 23:45:04 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bbcd4d40-525f-413c-85ef-29022ddec372",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22286c02-b986-4273-a77b-74dac1fc92ea",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-06-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1a390012-c886-4d2e-ac6b-9e36a86677b2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "890dbe88-153c-476e-877e-bcf84643b099",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-07-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "377ddb78-c87c-4d1f-a237-26328ca7d608",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "1422d14d-d0c1-4cdf-b64e-e20111b05d29",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-08-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bfff029c-d810-421b-9981-cbf978c27d00",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da06abee-1dfd-4714-8bda-1e841c25520b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-09-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "294eac1a-4bbe-4704-a9fc-9dba35067d29",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b71c06b1-378e-49e4-8182-707db81cb349",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-10-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "494878f0-fed3-4c03-9f81-739a2373f0a0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3d17a41f-f381-4118-8a9f-d3e355afa9f2",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-11-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "493a29c0-0489-43ee-b899-8d600fedc13b",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35584414-69a9-48eb-b2ff-058deaf81221",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-12-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "5d1e7a80-810a-4352-93da-8bd351356931",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd5376a6-394c-4f40-849c-f6e2ec8d67f7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-01-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "abc2d86a-7cae-4d57-ab23-922f919f6270",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "70845d12-71f7-446a-91aa-ad239610161b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-02-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "56ac1ddf-2e88-41aa-9ba3-dee1fcab07b9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5891fda-6435-465f-8d38-13029b58cc1c",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-03-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2593880d-08db-467b-9c0e-6f69514ea8d3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "39ea5154-7f7f-4562-b0b1-8d0ff137faf0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-04-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ef71066e-161f-4123-b271-b1bcded032f0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "162967ad-700e-49a2-b424-dec2d5756466",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-05-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/e26c22d7-9044-421e-901d-811b6f451058/reminders",
          "responseTime": 6,
          "duration": 6,
          "size": 8376
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "pgrnsgbc_s2Say2_O0yFM"
          },
          {
            "description": "should contain created reminders",
            "status": "pass",
            "uid": "nQ0nKUhDpUisaQy03PUw-"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.125569875,
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
          "url": "http://localhost:8080/leases/f8f3df33-2fa4-4aa4-ad97-7372eb9d73df/reminders",
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
            "x-request-id": "401e977c-72e5-4550-ab69-77935b6b660e",
            "date": "Wed, 17 Jun 2026 23:45:04 GMT",
            "content-length": "1418"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/f8f3df33-2fa4-4aa4-ad97-7372eb9d73df/reminders",
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
            "uid": "8PSxaq0cpfORjfxhdbSvn"
          },
          {
            "description": "should return array",
            "status": "pass",
            "uid": "_2bYCJRDCaUTjKM6_CG9a"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.121128709,
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
            "x-request-id": "3c7ebd79-2706-424b-a89e-d63be171bb51",
            "date": "Wed, 17 Jun 2026 23:45:04 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:02.223035+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:02.599338+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bbcd4d40-525f-413c-85ef-29022ddec372",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22286c02-b986-4273-a77b-74dac1fc92ea",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-06-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
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
                "scheduled_at": "2026-06-22T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.038276+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.846591+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:02.599338+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1a390012-c886-4d2e-ac6b-9e36a86677b2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "890dbe88-153c-476e-877e-bcf84643b099",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-07-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "377ddb78-c87c-4d1f-a237-26328ca7d608",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "1422d14d-d0c1-4cdf-b64e-e20111b05d29",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-08-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bfff029c-d810-421b-9981-cbf978c27d00",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da06abee-1dfd-4714-8bda-1e841c25520b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-09-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "294eac1a-4bbe-4704-a9fc-9dba35067d29",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b71c06b1-378e-49e4-8182-707db81cb349",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-10-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "494878f0-fed3-4c03-9f81-739a2373f0a0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3d17a41f-f381-4118-8a9f-d3e355afa9f2",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-11-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "493a29c0-0489-43ee-b899-8d600fedc13b",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35584414-69a9-48eb-b2ff-058deaf81221",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-12-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "5d1e7a80-810a-4352-93da-8bd351356931",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd5376a6-394c-4f40-849c-f6e2ec8d67f7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-01-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "abc2d86a-7cae-4d57-ab23-922f919f6270",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "70845d12-71f7-446a-91aa-ad239610161b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-02-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "56ac1ddf-2e88-41aa-9ba3-dee1fcab07b9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5891fda-6435-465f-8d38-13029b58cc1c",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-03-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2593880d-08db-467b-9c0e-6f69514ea8d3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "39ea5154-7f7f-4562-b0b1-8d0ff137faf0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-04-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ef71066e-161f-4123-b271-b1bcded032f0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "162967ad-700e-49a2-b424-dec2d5756466",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-05-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              }
            ]
          },
          "url": "http://localhost/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 13312
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "8DZ5fLQnwOI2u1NBe8M0_"
          },
          {
            "description": "should return array",
            "status": "pass",
            "uid": "-1iKeSaOJaEXpEimdattQ"
          },
          {
            "description": "should include created reminders",
            "status": "pass",
            "uid": "Um0sqbyWEwAbiK4GOY6Hn"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.120934792,
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
          "url": "http://localhost:8080/reminders/53ee5608-6397-4f55-a9e7-14b7e63a45e0",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-27\"\n}"
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
            "x-request-id": "ac81062c-326b-429b-a395-9e55c89cd65f",
            "date": "Wed, 17 Jun 2026 23:45:04 GMT",
            "content-length": "655"
          },
          "data": {
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
            "scheduled_at": "2026-06-27T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-18T02:45:04.038276+03:00"
          },
          "url": "http://localhost/reminders/53ee5608-6397-4f55-a9e7-14b7e63a45e0",
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
            "uid": "ucjF9F0B49nLNwjc3ioXS"
          },
          {
            "description": "should have same id",
            "status": "pass",
            "uid": "tpmSzxrBEhngb06vCoAS9"
          },
          {
            "description": "should still be pending",
            "status": "pass",
            "uid": "Z2nLmYGxR_j2LZCo4oBus"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.122986167,
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
            "x-request-id": "d957234a-b808-482c-8001-4e34833a44d1",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "session required",
            "requestId": "d957234a-b808-482c-8001-4e34833a44d1",
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
            "uid": "6opRUcMhbGdIRQ2Y5CGCZ"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.118511875,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/00000000-0000-0000-0000-000000000000/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-22\"\n}"
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
            "x-request-id": "30393722-d6f3-4501-b5b2-29202876ceba",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "operation not found",
            "requestId": "30393722-d6f3-4501-b5b2-29202876ceba",
            "status": 404,
            "title": "Not found",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/00000000-0000-0000-0000-000000000000/reminders",
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
            "uid": "QXZma6R4c8Vzgx351e_bD"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.120101834,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/11111111-1111-1111-1111-111111111111/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-22\"\n}"
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
            "x-request-id": "ac96eea3-1e87-48c0-81f2-8ca22141fcb9",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "operation not found",
            "requestId": "ac96eea3-1e87-48c0-81f2-8ca22141fcb9",
            "status": 404,
            "title": "Not found",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/11111111-1111-1111-1111-111111111111/reminders",
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
            "uid": "-rCBAlgwrTQiCBRnsLEtf"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.120019042,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/e26c22d7-9044-421e-901d-811b6f451058/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-16\"\n}"
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
            "x-request-id": "9908f777-15c4-483f-8e67-4b2bbff2a38f",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT",
            "content-length": "165"
          },
          "data": {
            "detail": "reminder date must be today or in the future",
            "requestId": "9908f777-15c4-483f-8e67-4b2bbff2a38f",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/e26c22d7-9044-421e-901d-811b6f451058/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 165
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "HKq1ljQyLC0-QBhkJw6XM"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.1158715,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/f3ec10d9-f054-4b1c-8387-93ef2d196131/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-22\"\n}"
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
            "x-request-id": "7907cdcb-076f-4139-959b-56092d625f3d",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT",
            "content-length": "154"
          },
          "data": {
            "detail": "no future operations for reminder",
            "requestId": "7907cdcb-076f-4139-959b-56092d625f3d",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/f3ec10d9-f054-4b1c-8387-93ef2d196131/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 154
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "4RXeUHCaVmcRlpaOaJdhY"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.120059625,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/36495d17-732b-4016-8d2c-4eecdbd46bf1/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-22\"\n}"
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
            "x-request-id": "ef0eebd3-9f60-4bbd-b7e7-0de172ab34c6",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-17T23:45:05.613059Z",
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
            "scheduled_at": "2026-06-22T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-17T23:45:05.613059Z"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/36495d17-732b-4016-8d2c-4eecdbd46bf1/reminders",
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
            "uid": "lUr9LtDfxjRzCrev4JQwF"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "MfrMQsXGXNjYgiv94GPY_"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.171859333,
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
          "url": "http://localhost:8080/reminders/d5297052-c047-41c8-9507-d5fa7fdc7d00",
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
            "x-request-id": "87ec7dfc-1fdf-48f5-9508-9d2ab93d076a",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT"
          },
          "data": "",
          "url": "http://localhost/reminders/d5297052-c047-41c8-9507-d5fa7fdc7d00",
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
            "uid": "r63QaMjJs1dLTV_2WCgrp"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.120571833,
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
          "url": "http://localhost:8080/reminders/d5297052-c047-41c8-9507-d5fa7fdc7d00",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-07-02\"\n}"
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
            "x-request-id": "bfdc9e7b-7490-44b3-b9e1-411c01295d11",
            "date": "Wed, 17 Jun 2026 23:45:05 GMT",
            "content-length": "144"
          },
          "data": {
            "detail": "reminder is not pending",
            "requestId": "bfdc9e7b-7490-44b3-b9e1-411c01295d11",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/reminders/d5297052-c047-41c8-9507-d5fa7fdc7d00",
          "responseTime": 5,
          "duration": 5,
          "size": 144
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400 or 409",
            "status": "pass",
            "uid": "Kszg6Xm7B33xbERbdSBI1"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.120560458,
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
          "data": "{\n  \"reminder_date\": \"2026-06-22\"\n}"
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
            "x-request-id": "056351ed-57f0-410b-bc4e-a25e005dd25a",
            "date": "Wed, 17 Jun 2026 23:45:06 GMT",
            "content-length": "137"
          },
          "data": {
            "detail": "reminder not found",
            "requestId": "056351ed-57f0-410b-bc4e-a25e005dd25a",
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
            "uid": "NAZtfg3k7D6lpGO55sify"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.12080225,
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
            "x-request-id": "02e6fe56-a44a-486e-bf8f-1b5d6cefcbfd",
            "date": "Wed, 17 Jun 2026 23:45:06 GMT",
            "content-length": "238"
          },
          "data": {
            "detail": "Invalid format for parameter limit: error binding string parameter: strconv.ParseInt: parsing \"abc\": invalid syntax",
            "requestId": "02e6fe56-a44a-486e-bf8f-1b5d6cefcbfd",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/reminders?limit=abc&offset=def",
          "responseTime": 2,
          "duration": 2,
          "size": 238
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "wby2rYUMDBkAzvrIheVIP"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.117786417,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/36495d17-732b-4016-8d2c-4eecdbd46bf1/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-17\"\n}"
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
            "x-request-id": "caf9955a-1aca-4105-bb45-21202ce08370",
            "date": "Wed, 17 Jun 2026 23:45:06 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-17T23:45:06.268705Z",
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
            "scheduled_at": "2026-06-17T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-17T23:45:06.268705Z"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/36495d17-732b-4016-8d2c-4eecdbd46bf1/reminders",
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
            "uid": "17m2tIK1kPBroEHDl5C7w"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "PpC2dX5m0evEoWQZoPo9k"
          },
          {
            "description": "should be pending",
            "status": "pass",
            "uid": "STj61RMiYWmmR9YdkLLfD"
          },
          {
            "description": "should be operation_due event",
            "status": "pass",
            "uid": "XtRM4_uHslDPffjb79slq"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.183824333,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
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
            "x-request-id": "187c005f-519c-41a6-acb8-7082acd5c93f",
            "date": "Wed, 17 Jun 2026 23:45:06 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "6a908986-3467-4ca3-9738-6d9b45c056fe",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "a2f5bb80-c1c2-4d84-a28b-4f5e2cff691f",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "04a465fc-9898-441b-b5bd-9a283875e278",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6d5ea349-02a9-44fc-8f7a-267929fbfb87",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "38c151c9-3f30-42bd-b6b2-0c2b9dc384b7",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c8adf04c-3cb2-4b41-b5c2-7925e752bada",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-08-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "b65fac02-2f89-4177-bbc1-099d1b7f8ee5",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "0087cf3a-692b-4825-9490-b680315557e6",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-09-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "50e32abf-dc16-47a0-8357-6807835a333a",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "7e1513bd-f29d-4a98-a5fc-93f14c2766a0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-10-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "05f2ee24-9b98-4bdd-9929-acc061c1ddfd",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac69b8b2-4814-45a2-bd45-5648339fa497",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-11-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d62fd6e8-a8c5-4969-b0ed-4078fd95e89e",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d3e378e7-a8c6-41a5-84e3-bdd6a613037d",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-12-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "f396b241-6d0a-4e79-a3cb-71ac59e9d346",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "534b7e31-671f-4917-91e6-6ab83fdc348b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-01-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1e1c0e32-3546-4163-9843-966816088ed1",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "64c34fda-c214-4beb-a429-f9be814a0c04",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-02-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "fc445552-7cb7-4e82-b057-b9e57cb0e1c1",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2bfeebe4-7d2d-4e17-b976-440dcd328646",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-03-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "c71553d5-5faf-4cf9-9cf4-30bbcf02fbe7",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f65c040b-9389-4190-a69c-a81a70cc6ec3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-04-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2a8994c8-aea6-4338-a135-39bcdd371ba0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "03a504ed-ff0a-4458-b89e-d891136fb419",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-05-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
          "responseTime": 15,
          "duration": 15,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "Bp4r-R-Jobvo9y9uEdhHe"
          },
          {
            "description": "should return reminders",
            "status": "pass",
            "uid": "8ztycJTpuqKwEoHS1iHZo"
          },
          {
            "description": "reminders should be pending",
            "status": "pass",
            "uid": "9CIY4E5gvCUrzYRZlFVlD"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.190525291,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-08-16\"\n}"
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
            "x-request-id": "6419c1ae-5e41-49ec-b37b-09b36c03930e",
            "date": "Wed, 17 Jun 2026 23:45:06 GMT",
            "content-length": "190"
          },
          "data": {
            "detail": "reminder date must be on or before the earliest future operation date",
            "requestId": "6419c1ae-5e41-49ec-b37b-09b36c03930e",
            "status": 400,
            "title": "Bad request",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
          "responseTime": 5,
          "duration": 5,
          "size": 190
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 400",
            "status": "pass",
            "uid": "-Pakut9SP5srK1RrNdVY8"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.122911792,
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
            "x-request-id": "2e53ea34-a3e3-460e-89c8-8b10898de071",
            "date": "Wed, 17 Jun 2026 23:45:06 GMT",
            "content-length": "686"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
              }
            ]
          },
          "url": "http://localhost/reminders?limit=0&offset=0",
          "responseTime": 2,
          "duration": 2,
          "size": 686
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "lGg8d7RD9XYZ9YcCRQejQ"
          },
          {
            "description": "should return array with at most one item",
            "status": "pass",
            "uid": "xu1h0JTcrzz_k3H4oyCox"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.119802833,
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
            "x-request-id": "b7cf4f75-7c53-48eb-bea4-39235fabe3b3",
            "date": "Wed, 17 Jun 2026 23:45:06 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:02.599338+03:00"
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
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.267924+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:02.223035+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "6a908986-3467-4ca3-9738-6d9b45c056fe",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "a2f5bb80-c1c2-4d84-a28b-4f5e2cff691f",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bbcd4d40-525f-413c-85ef-29022ddec372",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "22286c02-b986-4273-a77b-74dac1fc92ea",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-06-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
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
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.899162+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.846591+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:02.599338+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "04a465fc-9898-441b-b5bd-9a283875e278",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6d5ea349-02a9-44fc-8f7a-267929fbfb87",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:01.477173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1a390012-c886-4d2e-ac6b-9e36a86677b2",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "890dbe88-153c-476e-877e-bcf84643b099",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-07-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "38c151c9-3f30-42bd-b6b2-0c2b9dc384b7",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c8adf04c-3cb2-4b41-b5c2-7925e752bada",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-08-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "377ddb78-c87c-4d1f-a237-26328ca7d608",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "1422d14d-d0c1-4cdf-b64e-e20111b05d29",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-08-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "b65fac02-2f89-4177-bbc1-099d1b7f8ee5",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "0087cf3a-692b-4825-9490-b680315557e6",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-09-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "bfff029c-d810-421b-9981-cbf978c27d00",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "da06abee-1dfd-4714-8bda-1e841c25520b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-09-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "50e32abf-dc16-47a0-8357-6807835a333a",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "7e1513bd-f29d-4a98-a5fc-93f14c2766a0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-10-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "294eac1a-4bbe-4704-a9fc-9dba35067d29",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b71c06b1-378e-49e4-8182-707db81cb349",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-10-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "05f2ee24-9b98-4bdd-9929-acc061c1ddfd",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac69b8b2-4814-45a2-bd45-5648339fa497",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-11-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "494878f0-fed3-4c03-9f81-739a2373f0a0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "3d17a41f-f381-4118-8a9f-d3e355afa9f2",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-11-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "d62fd6e8-a8c5-4969-b0ed-4078fd95e89e",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d3e378e7-a8c6-41a5-84e3-bdd6a613037d",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-12-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "493a29c0-0489-43ee-b899-8d600fedc13b",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "35584414-69a9-48eb-b2ff-058deaf81221",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2026-12-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "f396b241-6d0a-4e79-a3cb-71ac59e9d346",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "534b7e31-671f-4917-91e6-6ab83fdc348b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-01-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "5d1e7a80-810a-4352-93da-8bd351356931",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "fd5376a6-394c-4f40-849c-f6e2ec8d67f7",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-01-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1e1c0e32-3546-4163-9843-966816088ed1",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "64c34fda-c214-4beb-a429-f9be814a0c04",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-02-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "abc2d86a-7cae-4d57-ab23-922f919f6270",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "70845d12-71f7-446a-91aa-ad239610161b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-02-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "fc445552-7cb7-4e82-b057-b9e57cb0e1c1",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2bfeebe4-7d2d-4e17-b976-440dcd328646",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-03-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "56ac1ddf-2e88-41aa-9ba3-dee1fcab07b9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "b5891fda-6435-465f-8d38-13029b58cc1c",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-03-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "c71553d5-5faf-4cf9-9cf4-30bbcf02fbe7",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f65c040b-9389-4190-a69c-a81a70cc6ec3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-04-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2593880d-08db-467b-9c0e-6f69514ea8d3",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "39ea5154-7f7f-4562-b0b1-8d0ff137faf0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-04-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:06.455678+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "2a8994c8-aea6-4338-a135-39bcdd371ba0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "03a504ed-ff0a-4458-b89e-d891136fb419",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-05-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:06.453339+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:04.344128+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "ef71066e-161f-4123-b271-b1bcded032f0",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "162967ad-700e-49a2-b424-dec2d5756466",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "e26c22d7-9044-421e-901d-811b6f451058",
                "scheduled_at": "2027-05-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:04.33913+03:00"
              }
            ]
          },
          "url": "http://localhost/reminders?limit=1000&offset=0",
          "responseTime": 3,
          "duration": 3,
          "size": 23010
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "lSeG3NB7QxtPC5EvmVDUy"
          },
          {
            "description": "should return array",
            "status": "pass",
            "uid": "35S5HilYYacTBLspHrZES"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.121877459,
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
            "x-request-id": "d41f16ad-9aa7-4ba3-96b3-e936ccb0b448",
            "date": "Wed, 17 Jun 2026 23:45:07 GMT",
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
            "uid": "Kij5VmlIthpWx9FtzJfYR"
          },
          {
            "description": "should return empty array",
            "status": "pass",
            "uid": "co_J6hyyFBVkK6_5rxfgr"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.121313416,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
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
            "x-request-id": "bd4770ef-0d62-4f99-862b-9c2d4f7db9f2",
            "date": "Wed, 17 Jun 2026 23:45:07 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "90573754-5f7e-43a5-b2df-63bd91d2afca",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "a2f5bb80-c1c2-4d84-a28b-4f5e2cff691f",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-06-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e1c9b105-9388-4a7e-873d-53c6d483c6f8",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6d5ea349-02a9-44fc-8f7a-267929fbfb87",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-07-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "429bbc34-ad36-4067-bf1e-4f9c423c00dd",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c8adf04c-3cb2-4b41-b5c2-7925e752bada",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-08-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "a2c016dc-7344-4888-bb3a-e3453f4790d5",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "0087cf3a-692b-4825-9490-b680315557e6",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-09-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "5f2f55de-b230-4194-88eb-38cc5d6ba1f6",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "7e1513bd-f29d-4a98-a5fc-93f14c2766a0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-10-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "afe0f8b1-40b4-4ade-87e7-26970fbb8be7",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac69b8b2-4814-45a2-bd45-5648339fa497",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-11-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "06adff33-0448-4475-8d2d-936eba164a24",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d3e378e7-a8c6-41a5-84e3-bdd6a613037d",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-12-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "f6cc96b6-861f-46e9-8816-90838ead874a",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "534b7e31-671f-4917-91e6-6ab83fdc348b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-01-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "cbd7ac3e-e5a5-40ce-8489-a3df3a36391f",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "64c34fda-c214-4beb-a429-f9be814a0c04",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-02-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "503ea094-c912-4990-9575-a67af26e5440",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2bfeebe4-7d2d-4e17-b976-440dcd328646",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-03-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "9907ef13-b82c-40ee-b83b-210506e4a817",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f65c040b-9389-4190-a69c-a81a70cc6ec3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-04-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.133694+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "109eb0b9-52bc-42d5-aa24-b38aba71e5c1",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "03a504ed-ff0a-4458-b89e-d891136fb419",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-05-18T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.130445+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
          "responseTime": 14,
          "duration": 14,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "UkgS-mE_W8fQPHT8XXpe2"
          },
          {
            "description": "should return reminders",
            "status": "pass",
            "uid": "nAVJt-NG7lMhj2B7RQA1I"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.189658583,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-20\"\n}"
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
            "x-request-id": "393d47af-c8bf-4e3e-a5b8-cc7f37f75571",
            "date": "Wed, 17 Jun 2026 23:45:07 GMT",
            "transfer-encoding": "chunked"
          },
          "data": {
            "items": [
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "1c34a512-68be-480b-bd85-13bc54e534c4",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.06.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "a2f5bb80-c1c2-4d84-a28b-4f5e2cff691f",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-06-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "7b15419b-950d-4393-9194-43e991bbaf31",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.07.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "6d5ea349-02a9-44fc-8f7a-267929fbfb87",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-07-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "e0756bd7-3fde-4d9f-b794-d1c61d2d3b6a",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.08.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "c8adf04c-3cb2-4b41-b5c2-7925e752bada",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-08-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "46bc437e-54ce-4c2e-b80f-1d513e6c556c",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.09.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "0087cf3a-692b-4825-9490-b680315557e6",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-09-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "926934c9-987b-4ae0-88c2-66c38df2ba73",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.10.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "7e1513bd-f29d-4a98-a5fc-93f14c2766a0",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-10-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "c4e0f952-5c72-42aa-a44c-1c0fb2ff77dc",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.11.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "ac69b8b2-4814-45a2-bd45-5648339fa497",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-11-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "922a82f8-ef77-4977-a265-1b040df06c62",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.12.2026",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "d3e378e7-a8c6-41a5-84e3-bdd6a613037d",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2026-12-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "98bc8179-bce8-4051-9a13-0abfccb04687",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.01.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "534b7e31-671f-4917-91e6-6ab83fdc348b",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-01-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "fabe3c37-69a4-44af-820d-9eb0c21c7435",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.02.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "64c34fda-c214-4beb-a429-f9be814a0c04",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-02-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "b3fd182f-4457-4232-80bc-5a9b8f22d6f9",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.03.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "2bfeebe4-7d2d-4e17-b976-440dcd328646",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-03-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "eb2ff319-24a3-4590-aa28-ecc07ec0b787",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.04.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "f65c040b-9389-4190-a69c-a81a70cc6ec3",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-04-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              },
              {
                "created_at": "2026-06-18T02:45:07.322192+03:00",
                "event_type": "operation_due",
                "failed_attempts": 0,
                "id": "4273b56a-68a3-43b8-bc56-5236deacec3c",
                "lease_id": null,
                "message_body": "utilities 500.00 ₽ запланировано на 20.05.2027",
                "message_title": "Напоминание об операции",
                "next_attempt_at": null,
                "operation_id": "03a504ed-ff0a-4458-b89e-d891136fb419",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
                "recurring_operation_id": "2475d791-8e59-48aa-a991-46da23d13bf7",
                "scheduled_at": "2027-05-20T10:00:00+03:00",
                "sent_at": null,
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:07.319173+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/recurring-operations/2475d791-8e59-48aa-a991-46da23d13bf7/reminders",
          "responseTime": 15,
          "duration": 15,
          "size": 8388
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "rq-ENVcCZae7ahoVDgf9d"
          },
          {
            "description": "should return new reminders",
            "status": "pass",
            "uid": "3iwUmAbf1mtBG8a9-X3zJ"
          },
          {
            "description": "old reminders should be replaced",
            "status": "pass",
            "uid": "xhhgvaSUNJdBGj2AJ6aXl"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.189370708,
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
          "url": "http://localhost:8080/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"operation_date\": \"2026-06-27\",\n  \"comment\": \"E2E manual operation main updated\"\n}"
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
            "x-request-id": "5bef8b43-1911-4f4d-bfe2-82637419c776",
            "date": "Wed, 17 Jun 2026 23:45:07 GMT",
            "content-length": "445"
          },
          "data": {
            "amount_kopecks": 150000,
            "category": "rent",
            "comment": "E2E manual operation main updated",
            "created_at": "2026-06-18T02:45:00.920389+03:00",
            "id": "9b53d867-6cd6-440e-b63a-02a8c63a53db",
            "is_exception": true,
            "lease_id": null,
            "operation_date": "2026-06-27",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "property_id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "recurring_operation_id": null,
            "type": "income",
            "updated_at": "2026-06-18T02:45:07.506876+03:00"
          },
          "url": "http://localhost/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db",
          "responseTime": 9,
          "duration": 9,
          "size": 445
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "Re5KgAg9icCO51MEj1iR2"
          },
          {
            "description": "should have updated date",
            "status": "pass",
            "uid": "W3L60j5JcL3Jof9dVBg1g"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.126703917,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db/reminders",
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
            "x-request-id": "c39f7e9c-ca0c-4bd1-947c-dec8d516d691",
            "date": "Wed, 17 Jun 2026 23:45:07 GMT",
            "content-length": "672"
          },
          "data": {
            "items": [
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
              }
            ]
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/9b53d867-6cd6-440e-b63a-02a8c63a53db/reminders",
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
            "uid": "KuLMPaZwCyeRUtYUjh8gh"
          },
          {
            "description": "should have a pending reminder",
            "status": "pass",
            "uid": "FVHy8h3LQULJkY78x3YWE"
          },
          {
            "description": "reminder should be operation_due",
            "status": "pass",
            "uid": "CgzTP_AVFCbYiQo9kzGq7"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.1237115,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/e345ebe2-3958-4d12-8ec0-47ca8064927c/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-22\"\n}"
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
            "x-request-id": "a98c10cc-511c-4b33-abfb-b55498689424",
            "date": "Wed, 17 Jun 2026 23:45:07 GMT",
            "content-length": "645"
          },
          "data": {
            "created_at": "2026-06-17T23:45:07.758107Z",
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
            "scheduled_at": "2026-06-22T07:00:00Z",
            "sent_at": null,
            "status": "pending",
            "target_type": "operation",
            "updated_at": "2026-06-17T23:45:07.758107Z"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/e345ebe2-3958-4d12-8ec0-47ca8064927c/reminders",
          "responseTime": 4,
          "duration": 4,
          "size": 645
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "hWJcU29rmDe8UF4_Z7lI5"
          },
          {
            "description": "should have reminder id",
            "status": "pass",
            "uid": "eFNvXMR0vsqWzRquANfha"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.177128291,
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
          "url": "http://localhost:8080/operations/e345ebe2-3958-4d12-8ec0-47ca8064927c",
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
            "x-request-id": "8dc6f17b-4637-4289-b7d2-0613715253bb",
            "date": "Wed, 17 Jun 2026 23:45:07 GMT"
          },
          "data": "",
          "url": "http://localhost/operations/e345ebe2-3958-4d12-8ec0-47ca8064927c",
          "responseTime": 6,
          "duration": 6,
          "size": 0
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 204",
            "status": "pass",
            "uid": "UC0vKTvJiY_9Q3RGPENK6"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.122059833,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/e345ebe2-3958-4d12-8ec0-47ca8064927c/reminders",
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
            "x-request-id": "50751ab5-81b9-443d-b988-ff33bb06c1ed",
            "date": "Wed, 17 Jun 2026 23:45:08 GMT",
            "content-length": "138"
          },
          "data": {
            "detail": "operation not found",
            "requestId": "50751ab5-81b9-443d-b988-ff33bb06c1ed",
            "status": 404,
            "title": "Not found",
            "type": "about:blank"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/operations/e345ebe2-3958-4d12-8ec0-47ca8064927c/reminders",
          "responseTime": 2,
          "duration": 2,
          "size": 138
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 404 (operation deleted)",
            "status": "pass",
            "uid": "BqCLz5F3jhLD1TTEFrv1w"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.11763125,
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
          "url": "http://localhost:8080/leases/434602af-3b0b-4e51-a0d1-04ea03638bbe",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"end_date\": \"2026-08-01\",\n  \"comment\": \"E2E lease lifecycle updated\"\n}"
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
            "x-request-id": "29eb3077-b525-4d53-9a19-4a81439612c9",
            "date": "Wed, 17 Jun 2026 23:45:08 GMT",
            "content-length": "800"
          },
          "data": {
            "comment": "E2E lease lifecycle updated",
            "created_at": "2026-06-18T02:45:02.599338+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-08-01",
            "id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:08.174052+03:00"
          },
          "url": "http://localhost/leases/434602af-3b0b-4e51-a0d1-04ea03638bbe",
          "responseTime": 16,
          "duration": 16,
          "size": 800
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "kphLgI_NMGPO8EfnPrH0Y"
          },
          {
            "description": "should have updated end date",
            "status": "pass",
            "uid": "lMwW23vclWyp3_dzxMO1Q"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.132795459,
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
          "url": "http://localhost:8080/leases/434602af-3b0b-4e51-a0d1-04ea03638bbe/reminders",
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
            "x-request-id": "39c97213-9b97-4c56-a1ee-f70bc9e595e3",
            "date": "Wed, 17 Jun 2026 23:45:08 GMT",
            "content-length": "1414"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:08.174052+03:00"
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
                "status": "pending",
                "target_type": "lease",
                "updated_at": "2026-06-18T02:45:08.174052+03:00"
              }
            ]
          },
          "url": "http://localhost/leases/434602af-3b0b-4e51-a0d1-04ea03638bbe/reminders",
          "responseTime": 5,
          "duration": 5,
          "size": 1414
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "2LfcUwyry9NQUkT1JiA6Q"
          },
          {
            "description": "should contain lease_expiring reminder",
            "status": "pass",
            "uid": "9W5NRnrqSWN5z92SwbRqO"
          },
          {
            "description": "should contain requires_action reminder",
            "status": "pass",
            "uid": "3AqCyUHM_G3_QLhuAHNoG"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.176709958,
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
          "data": "{\n  \"name\": \"E2E Complete 1781739908481\",\n  \"type\": \"apartment\",\n  \"address\": \"Complete St 1781739908481\",\n  \"description\": \"E2E reminders complete property\"\n}"
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
            "x-request-id": "84126646-c1da-411a-aca3-af4bc2ddabd7",
            "date": "Wed, 17 Jun 2026 23:45:08 GMT",
            "content-length": "320"
          },
          "data": {
            "address": "Complete St 1781739908481",
            "created_at": "2026-06-18T02:45:08.483722+03:00",
            "description": "E2E reminders complete property",
            "id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
            "name": "E2E Complete 1781739908481",
            "occupancy": "free",
            "status": "active",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:08.483722+03:00"
          },
          "url": "http://localhost/properties",
          "responseTime": 7,
          "duration": 7,
          "size": 320
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "JniAKAJaTAR_pDmpOHA__"
          },
          {
            "description": "should have property id",
            "status": "pass",
            "uid": "cVVYdX3mHA6Pd8HtbdhQX"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.180781,
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
          "data": "{\n  \"property_id\": \"3622b901-b75f-4205-a5e6-b37a6da3da1a\",\n  \"tenant_contact_id\": \"9d1bb26e-d63c-4606-ba90-9e7c7ab359f7\",\n  \"start_date\": \"2026-06-17\",\n  \"end_date\": \"2026-07-17\",\n  \"rent_amount_kopecks\": 5000000,\n  \"payment_day\": 20,\n  \"deposit_amount_kopecks\": 1000000,\n  \"comment\": \"E2E lease complete\"\n}"
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
            "x-request-id": "2d8d1445-5fd5-419d-bb48-4004eeca5cd6",
            "date": "Wed, 17 Jun 2026 23:45:08 GMT",
            "content-length": "792"
          },
          "data": {
            "comment": "E2E lease complete",
            "created_at": "2026-06-18T02:45:08.666278+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-17",
            "id": "eb03273a-e10a-4fac-b9c0-b98e27517527",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 20,
            "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "active",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:08.666278+03:00"
          },
          "url": "http://localhost/leases",
          "responseTime": 11,
          "duration": 11,
          "size": 792
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "T7wZHET5iW5NnG_6WGE9j"
          },
          {
            "description": "should have lease id",
            "status": "pass",
            "uid": "fCzwaHC34rCQQv7z--G7U"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.18398275,
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
          "url": "http://localhost:8080/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/recurring-operations",
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
            "x-request-id": "0ac83cf7-e919-4f8c-af20-925d07e2c053",
            "date": "Wed, 17 Jun 2026 23:45:08 GMT",
            "content-length": "490"
          },
          "data": {
            "items": [
              {
                "amount_kopecks": 5000000,
                "category": "rent",
                "comment": null,
                "created_at": "2026-06-18T02:45:08.666278+03:00",
                "end_date": "2026-07-17",
                "id": "326693b1-9ba5-42a2-b772-7a6e68f967af",
                "lease_id": "eb03273a-e10a-4fac-b9c0-b98e27517527",
                "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
                "payment_day": 20,
                "periodicity": "monthly",
                "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
                "start_date": "2026-06-17",
                "status": "active",
                "type": "income",
                "updated_at": "2026-06-18T02:45:08.666278+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/recurring-operations",
          "responseTime": 5,
          "duration": 5,
          "size": 490
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "SlSIhX_xKSNQvwo8M-iam"
          },
          {
            "description": "should contain recurring operation",
            "status": "pass",
            "uid": "HiD32_3_fmaenXPUz5OUa"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.177913875,
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
          "url": "http://localhost:8080/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/recurring-operations/326693b1-9ba5-42a2-b772-7a6e68f967af/reminders",
          "headers": {
            "content-type": "application/json",
            "Cookie": "********"
          },
          "data": "{\n  \"reminder_date\": \"2026-06-17\"\n}"
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
            "x-request-id": "569e6ddf-6be0-4dae-a100-d5c5c8a8e0e8",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "707"
          },
          "data": {
            "items": [
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
                "status": "pending",
                "target_type": "operation",
                "updated_at": "2026-06-18T02:45:09.029421+03:00"
              }
            ]
          },
          "url": "http://localhost/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/recurring-operations/326693b1-9ba5-42a2-b772-7a6e68f967af/reminders",
          "responseTime": 10,
          "duration": 10,
          "size": 707
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 201",
            "status": "pass",
            "uid": "GTE1GRpECpBn2x_iMcfhi"
          },
          {
            "description": "should return reminders",
            "status": "pass",
            "uid": "sjLJggLPDV65LaRYhFBq1"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.187446417,
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
          "url": "http://localhost:8080/leases/eb03273a-e10a-4fac-b9c0-b98e27517527/complete",
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
            "x-request-id": "d013f88f-92e2-4a09-81c6-cc1fe59e87ed",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "795"
          },
          "data": {
            "comment": "E2E lease complete",
            "created_at": "2026-06-18T02:45:08.666278+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-17",
            "id": "eb03273a-e10a-4fac-b9c0-b98e27517527",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 20,
            "property_id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:09.213612+03:00"
          },
          "url": "http://localhost/leases/eb03273a-e10a-4fac-b9c0-b98e27517527/complete",
          "responseTime": 6,
          "duration": 6,
          "size": 795
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "L9r68sKnAmwwW8HxAnESX"
          },
          {
            "description": "should have matching id",
            "status": "pass",
            "uid": "mng9nJYXoRbSzEP6XGSLZ"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.121546209,
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
          "url": "http://localhost:8080/leases/eb03273a-e10a-4fac-b9c0-b98e27517527/reminders",
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
            "x-request-id": "e97c2a2f-e621-42d9-8f9e-7f2c788a1117",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "13"
          },
          "data": {
            "items": []
          },
          "url": "http://localhost/leases/eb03273a-e10a-4fac-b9c0-b98e27517527/reminders",
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
            "uid": "4HGJ8Z9IVXj9mv4nyOc7O"
          },
          {
            "description": "should have no lease reminders",
            "status": "pass",
            "uid": "327Ts-9EVRSP0TBYBbUJP"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.119418875,
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
          "url": "http://localhost:8080/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/recurring-operations/326693b1-9ba5-42a2-b772-7a6e68f967af/reminders",
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
            "x-request-id": "cbe4afed-1805-44f1-8473-b85a66fe4bd0",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "13"
          },
          "data": {
            "items": []
          },
          "url": "http://localhost/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/recurring-operations/326693b1-9ba5-42a2-b772-7a6e68f967af/reminders",
          "responseTime": 5,
          "duration": 5,
          "size": 13
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "jeBRCkz0LcYraC2mEN7_B"
          },
          {
            "description": "should not contain previous recurring reminder",
            "status": "pass",
            "uid": "FROxSTJINe9H6haA7ni8h"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.120367792,
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
          "url": "http://localhost:8080/leases/f8f3df33-2fa4-4aa4-ad97-7372eb9d73df/complete",
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
            "x-request-id": "b380027e-189b-40c1-b2bf-2e799cc82223",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "789"
          },
          "data": {
            "comment": "E2E lease 30d",
            "created_at": "2026-06-18T02:45:01.477173+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-17",
            "id": "f8f3df33-2fa4-4aa4-ad97-7372eb9d73df",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:09.576157+03:00"
          },
          "url": "http://localhost/leases/f8f3df33-2fa4-4aa4-ad97-7372eb9d73df/complete",
          "responseTime": 9,
          "duration": 9,
          "size": 789
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "fVjGe0BXxtipdlRPeHO6Z"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.1256045,
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
          "url": "http://localhost:8080/leases/e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489/complete",
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
            "x-request-id": "05cb905c-c5ca-4daa-a9c3-d7fe9d6932aa",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "791"
          },
          "data": {
            "comment": "E2E lease short",
            "created_at": "2026-06-18T02:45:01.846591+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-07-02",
            "id": "e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "44bc828a-fd1e-410c-95c7-73fc77034005",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:09.701155+03:00"
          },
          "url": "http://localhost/leases/e1ed1f0e-c1e2-4ec1-a6c3-e18c3f55f489/complete",
          "responseTime": 9,
          "duration": 9,
          "size": 791
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "DRevTDmN8BR3BWWKwYkGE"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.1248565,
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
          "url": "http://localhost:8080/leases/e873f094-d8eb-4736-a45c-586e9ef7833f/complete",
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
            "x-request-id": "3f2dfaed-eaa8-4af5-a3cb-5c2884c64840",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "790"
          },
          "data": {
            "comment": "E2E lease past",
            "created_at": "2026-06-18T02:45:02.223035+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-06-12",
            "id": "e873f094-d8eb-4736-a45c-586e9ef7833f",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-05-18",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:09.826213+03:00"
          },
          "url": "http://localhost/leases/e873f094-d8eb-4736-a45c-586e9ef7833f/complete",
          "responseTime": 9,
          "duration": 9,
          "size": 790
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "-_tjewVKBKirpaP907r_J"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.124725417,
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
          "url": "http://localhost:8080/leases/434602af-3b0b-4e51-a0d1-04ea03638bbe/complete",
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
            "x-request-id": "882e60e8-40d7-4094-8e33-695780541c26",
            "date": "Wed, 17 Jun 2026 23:45:09 GMT",
            "content-length": "803"
          },
          "data": {
            "comment": "E2E lease lifecycle updated",
            "created_at": "2026-06-18T02:45:02.599338+03:00",
            "deposit_amount_kopecks": 1000000,
            "end_date": "2026-08-01",
            "id": "434602af-3b0b-4e51-a0d1-04ea03638bbe",
            "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
            "payment_day": 1,
            "property_id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
            "rent_amount_kopecks": 5000000,
            "start_date": "2026-06-17",
            "status": "completed",
            "tenant_contact": {
              "comment": "Primary tenant contact",
              "created_at": "2026-06-18T02:45:00.735538+03:00",
              "email": "tenant-1781739900731@example.com",
              "id": "9d1bb26e-d63c-4606-ba90-9e7c7ab359f7",
              "name": "Ivan",
              "owner_id": "694d6db2-f61e-43db-8db2-0924aef1e99d",
              "patronymic": "Ivanovich",
              "phone": "+79159900731",
              "surname": "Ivanov",
              "updated_at": "2026-06-18T02:45:00.735538+03:00"
            },
            "updated_at": "2026-06-18T02:45:09.950693+03:00"
          },
          "url": "http://localhost/leases/434602af-3b0b-4e51-a0d1-04ea03638bbe/complete",
          "responseTime": 9,
          "duration": 9,
          "size": 803
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "TlQWUpIKSebCa5Hl-ciTb"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.125570917,
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
          "url": "http://localhost:8080/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/archive",
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
            "x-request-id": "02261f11-f578-4904-b4f5-543c13f6a7d1",
            "date": "Wed, 17 Jun 2026 23:45:10 GMT",
            "content-length": "321"
          },
          "data": {
            "address": "Main St 1781739900516",
            "created_at": "2026-06-18T02:45:00.549605+03:00",
            "description": "E2E reminders test property main",
            "id": "69610376-d896-4182-bfd3-bfc7819f7c8c",
            "name": "E2E Reminders Main 1781739900516",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:10.075861+03:00"
          },
          "url": "http://localhost/properties/69610376-d896-4182-bfd3-bfc7819f7c8c/archive",
          "responseTime": 14,
          "duration": 14,
          "size": 321
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "u7xiZeFTkahYS-9lMiWfD"
          },
          {
            "description": "should be archived",
            "status": "pass",
            "uid": "tcq_mECyIes8pyTcz13qH"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.130145417,
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
          "url": "http://localhost:8080/properties/f978f1c6-c69c-43f6-bc9f-b3e765d05089/archive",
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
            "x-request-id": "97742df9-b601-4b6e-8adf-f481362b884e",
            "date": "Wed, 17 Jun 2026 23:45:10 GMT",
            "content-length": "321"
          },
          "data": {
            "address": "Lease 30d St 1781739901287",
            "created_at": "2026-06-18T02:45:01.290568+03:00",
            "description": "E2E reminders lease 30d property",
            "id": "f978f1c6-c69c-43f6-bc9f-b3e765d05089",
            "name": "E2E Lease 30d 1781739901287",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:10.206536+03:00"
          },
          "url": "http://localhost/properties/f978f1c6-c69c-43f6-bc9f-b3e765d05089/archive",
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
            "uid": "PZv3WohD8_aSVVGeug-WY"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.124741667,
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
          "url": "http://localhost:8080/properties/44bc828a-fd1e-410c-95c7-73fc77034005/archive",
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
            "x-request-id": "f49b59ec-3a5a-40a1-af41-3a19a3c020d7",
            "date": "Wed, 17 Jun 2026 23:45:10 GMT",
            "content-length": "326"
          },
          "data": {
            "address": "Lease Short St 1781739901660",
            "created_at": "2026-06-18T02:45:01.66242+03:00",
            "description": "E2E reminders lease short property",
            "id": "44bc828a-fd1e-410c-95c7-73fc77034005",
            "name": "E2E Lease Short 1781739901660",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:10.327199+03:00"
          },
          "url": "http://localhost/properties/44bc828a-fd1e-410c-95c7-73fc77034005/archive",
          "responseTime": 6,
          "duration": 6,
          "size": 326
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "TjEkqDUpC0z5-UDfEeWCa"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.119774208,
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
          "url": "http://localhost:8080/properties/3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96/archive",
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
            "x-request-id": "2658ebb4-2f24-490c-9093-289855f541eb",
            "date": "Wed, 17 Jun 2026 23:45:10 GMT",
            "content-length": "324"
          },
          "data": {
            "address": "Lease Past St 1781739902033",
            "created_at": "2026-06-18T02:45:02.035985+03:00",
            "description": "E2E reminders lease past property",
            "id": "3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96",
            "name": "E2E Lease Past 1781739902033",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:10.450372+03:00"
          },
          "url": "http://localhost/properties/3f4ea267-dc1e-48fb-9e4d-8d218a9b7f96/archive",
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
            "uid": "dRi1wsg7siCuN-pw2es52"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.123944125,
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
          "url": "http://localhost:8080/properties/bf10cc7e-c23b-4fbb-945e-4047b5b6cafa/archive",
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
            "x-request-id": "8f7cf734-57a8-4427-9509-8434c3a46641",
            "date": "Wed, 17 Jun 2026 23:45:10 GMT",
            "content-length": "321"
          },
          "data": {
            "address": "Lifecycle St 1781739902409",
            "created_at": "2026-06-18T02:45:02.412691+03:00",
            "description": "E2E reminders lifecycle property",
            "id": "bf10cc7e-c23b-4fbb-945e-4047b5b6cafa",
            "name": "E2E Lifecycle 1781739902409",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:10.572479+03:00"
          },
          "url": "http://localhost/properties/bf10cc7e-c23b-4fbb-945e-4047b5b6cafa/archive",
          "responseTime": 8,
          "duration": 8,
          "size": 321
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "-rioALSpvyquII0uo9bCC"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.122698417,
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
          "url": "http://localhost:8080/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/archive",
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
            "x-request-id": "5bad7a4f-4e95-436d-ba76-a642f5a442de",
            "date": "Wed, 17 Jun 2026 23:45:10 GMT",
            "content-length": "318"
          },
          "data": {
            "address": "Complete St 1781739908481",
            "created_at": "2026-06-18T02:45:08.483722+03:00",
            "description": "E2E reminders complete property",
            "id": "3622b901-b75f-4205-a5e6-b37a6da3da1a",
            "name": "E2E Complete 1781739908481",
            "occupancy": "",
            "status": "archived",
            "type": "apartment",
            "updated_at": "2026-06-18T02:45:10.697064+03:00"
          },
          "url": "http://localhost/properties/3622b901-b75f-4205-a5e6-b37a6da3da1a/archive",
          "responseTime": 7,
          "duration": 7,
          "size": 318
        },
        "error": null,
        "status": "pass",
        "assertionResults": [],
        "testResults": [
          {
            "description": "should return 200",
            "status": "pass",
            "uid": "Z1SdldocdLLx3GSLH_Ubh"
          }
        ],
        "preRequestTestResults": [],
        "postResponseTestResults": [],
        "shouldStopRunnerExecution": false,
        "runDuration": 0.123436125,
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
 cancelled_reminders       |           |  14
 failed_reminders          |           |   0
 sent_sms_audit_rows       |           |   1
 total_reminders_by_status | pending   |   1
 total_reminders_by_status | sent      |   1
 total_reminders_by_status | cancelled |  14
(6 rows)
```

## Recent Backend Log Excerpt

```
[2m02:45:04[0m [92mINF[0m request handled [2mrequest_id=[0m7c389c21-9c2e-42f5-a7ca-cd579b8788b3 [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m200 [2mduration=[0m2.479709ms
[2m02:45:04[0m [92mINF[0m request handled [2mrequest_id=[0m2dbebfa3-abeb-4377-ba8e-74226a244dd0 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m18.300083ms
[2m02:45:04[0m [92mINF[0m request handled [2mrequest_id=[0m614d6a0b-2b30-42dc-a45e-afb569047a14 [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m200 [2mduration=[0m4.404958ms
[2m02:45:04[0m [92mINF[0m request handled [2mrequest_id=[0m401e977c-72e5-4550-ab69-77935b6b660e [2mmethod=[0mGET [2mroute=[0m/leases/{leaseId}/reminders [2mstatus=[0m200 [2mduration=[0m3.137917ms
[2m02:45:04[0m [92mINF[0m request handled [2mrequest_id=[0m3c7ebd79-2706-424b-a89e-d63be171bb51 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m2.701875ms
[2m02:45:04[0m [92mINF[0m request handled [2mrequest_id=[0mac81062c-326b-429b-a395-9e55c89cd65f [2mmethod=[0mPATCH [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m200 [2mduration=[0m6.057292ms
[2m02:45:05[0m [93mWRN[0m request handled [2mrequest_id=[0md957234a-b808-482c-8001-4e34833a44d1 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m401 [2mduration=[0m47.959µs [2merror_code=[0mUnauthorized
[2m02:45:05[0m [93mWRN[0m request handled [2mrequest_id=[0m30393722-d6f3-4501-b5b2-29202876ceba [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m404 [2mduration=[0m1.93125ms [2merror_code=[0m"Not found"
[2m02:45:05[0m [93mWRN[0m request handled [2mrequest_id=[0mac96eea3-1e87-48c0-81f2-8ca22141fcb9 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m404 [2mduration=[0m2.530667ms [2merror_code=[0m"Not found"
[2m02:45:05[0m [93mWRN[0m request handled [2mrequest_id=[0m9908f777-15c4-483f-8e67-4b2bbff2a38f [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m400 [2mduration=[0m2.739584ms [2merror_code=[0m"Bad request"
[2m02:45:05[0m [93mWRN[0m request handled [2mrequest_id=[0m7907cdcb-076f-4139-959b-56092d625f3d [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m400 [2mduration=[0m3.120625ms [2merror_code=[0m"Bad request"
[2m02:45:05[0m [92mINF[0m request handled [2mrequest_id=[0mef0eebd3-9f60-4bbd-b7e7-0de172ab34c6 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m201 [2mduration=[0m3.548917ms
[2m02:45:05[0m [92mINF[0m request handled [2mrequest_id=[0m87ec7dfc-1fdf-48f5-9508-9d2ab93d076a [2mmethod=[0mDELETE [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m204 [2mduration=[0m3.939583ms
[2m02:45:05[0m [93mWRN[0m request handled [2mrequest_id=[0mbfdc9e7b-7490-44b3-b9e1-411c01295d11 [2mmethod=[0mPATCH [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m400 [2mduration=[0m2.169917ms [2merror_code=[0m"Bad request"
[2m02:45:06[0m [93mWRN[0m request handled [2mrequest_id=[0m056351ed-57f0-410b-bc4e-a25e005dd25a [2mmethod=[0mPATCH [2mroute=[0m/reminders/{reminderId} [2mstatus=[0m404 [2mduration=[0m2.45375ms [2merror_code=[0m"Not found"
[2m02:45:06[0m [93mWRN[0m request handled [2mrequest_id=[0m02e6fe56-a44a-486e-bf8f-1b5d6cefcbfd [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m400 [2mduration=[0m830.875µs [2merror_code=[0m"Bad request"
[2m02:45:06[0m [92mINF[0m request handled [2mrequest_id=[0mcaf9955a-1aca-4105-bb45-21202ce08370 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m201 [2mduration=[0m3.382125ms
[2m02:45:06[0m [92mINF[0m request handled [2mrequest_id=[0m187c005f-519c-41a6-acb8-7082acd5c93f [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m14.077958ms
[2m02:45:06[0m [93mWRN[0m request handled [2mrequest_id=[0m6419c1ae-5e41-49ec-b37b-09b36c03930e [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m400 [2mduration=[0m4.199583ms [2merror_code=[0m"Bad request"
[2m02:45:06[0m [92mINF[0m request handled [2mrequest_id=[0m2e53ea34-a3e3-460e-89c8-8b10898de071 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m1.365167ms
[2m02:45:06[0m [92mINF[0m request handled [2mrequest_id=[0mb7cf4f75-7c53-48eb-bea4-39235fabe3b3 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m2.327375ms
[2m02:45:07[0m [92mINF[0m request handled [2mrequest_id=[0md41f16ad-9aa7-4ba3-96b3-e936ccb0b448 [2mmethod=[0mGET [2mroute=[0m/reminders [2mstatus=[0m200 [2mduration=[0m2.728416ms
[2m02:45:07[0m [92mINF[0m request handled [2mrequest_id=[0mbd4770ef-0d62-4f99-862b-9c2d4f7db9f2 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m13.452875ms
[2m02:45:07[0m [92mINF[0m request handled [2mrequest_id=[0m393d47af-c8bf-4e3e-a5b8-cc7f37f75571 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m13.500042ms
[2m02:45:07[0m [92mINF[0m request handled [2mrequest_id=[0m5bef8b43-1911-4f4d-bfe2-82637419c776 [2mmethod=[0mPATCH [2mroute=[0m/operations/{id} [2mstatus=[0m200 [2mduration=[0m8.247584ms
[2m02:45:07[0m [92mINF[0m request handled [2mrequest_id=[0mc39f7e9c-ca0c-4bd1-947c-dec8d516d691 [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m200 [2mduration=[0m2.393958ms
[2m02:45:07[0m [92mINF[0m request handled [2mrequest_id=[0ma98c10cc-511c-4b33-abfb-b55498689424 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m201 [2mduration=[0m3.245042ms
[2m02:45:07[0m [92mINF[0m request handled [2mrequest_id=[0m8dc6f17b-4637-4289-b7d2-0613715253bb [2mmethod=[0mDELETE [2mroute=[0m/operations/{id} [2mstatus=[0m204 [2mduration=[0m4.780541ms
[2m02:45:08[0m [93mWRN[0m request handled [2mrequest_id=[0m50751ab5-81b9-443d-b988-ff33bb06c1ed [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/operations/{operationId}/reminders [2mstatus=[0m404 [2mduration=[0m1.172708ms [2merror_code=[0m"Not found"
[2m02:45:08[0m [92mINF[0m request handled [2mrequest_id=[0m29eb3077-b525-4d53-9a19-4a81439612c9 [2mmethod=[0mPATCH [2mroute=[0m/leases/{id} [2mstatus=[0m200 [2mduration=[0m14.105417ms
[2m02:45:08[0m [92mINF[0m request handled [2mrequest_id=[0m39c97213-9b97-4c56-a1ee-f70bc9e595e3 [2mmethod=[0mGET [2mroute=[0m/leases/{leaseId}/reminders [2mstatus=[0m200 [2mduration=[0m3.852959ms
[2m02:45:08[0m [92mINF[0m request handled [2mrequest_id=[0m84126646-c1da-411a-aca3-af4bc2ddabd7 [2mmethod=[0mPOST [2mroute=[0m/properties [2mstatus=[0m201 [2mduration=[0m6.014542ms
[2m02:45:08[0m [92mINF[0m request handled [2mrequest_id=[0m2d8d1445-5fd5-419d-bb48-4004eeca5cd6 [2mmethod=[0mPOST [2mroute=[0m/leases [2mstatus=[0m201 [2mduration=[0m9.621875ms
[2m02:45:08[0m [92mINF[0m request handled [2mrequest_id=[0m0ac83cf7-e919-4f8c-af20-925d07e2c053 [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/recurring-operations [2mstatus=[0m200 [2mduration=[0m3.82425ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0m569e6ddf-6be0-4dae-a100-d5c5c8a8e0e8 [2mmethod=[0mPOST [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m201 [2mduration=[0m8.38225ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0md013f88f-92e2-4a09-81c6-cc1fe59e87ed [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m4.672083ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0me97c2a2f-e621-42d9-8f9e-7f2c788a1117 [2mmethod=[0mGET [2mroute=[0m/leases/{leaseId}/reminders [2mstatus=[0m200 [2mduration=[0m2.130583ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0mcbe4afed-1805-44f1-8473-b85a66fe4bd0 [2mmethod=[0mGET [2mroute=[0m/properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders [2mstatus=[0m200 [2mduration=[0m3.376375ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0mb380027e-189b-40c1-b2bf-2e799cc82223 [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m7.889625ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0m05cb905c-c5ca-4daa-a9c3-d7fe9d6932aa [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m7.317ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0m3f2dfaed-eaa8-4af5-a3cb-5c2884c64840 [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m7.9865ms
[2m02:45:09[0m [92mINF[0m request handled [2mrequest_id=[0m882e60e8-40d7-4094-8e33-695780541c26 [2mmethod=[0mPOST [2mroute=[0m/leases/{id}/complete [2mstatus=[0m200 [2mduration=[0m8.114875ms
[2m02:45:10[0m [92mINF[0m request handled [2mrequest_id=[0m02261f11-f578-4904-b4f5-543c13f6a7d1 [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m12.962458ms
[2m02:45:10[0m [92mINF[0m request handled [2mrequest_id=[0m97742df9-b601-4b6e-8adf-f481362b884e [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m7.370042ms
[2m02:45:10[0m [92mINF[0m request handled [2mrequest_id=[0mf49b59ec-3a5a-40a1-af41-3a19a3c020d7 [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m5.144541ms
[2m02:45:10[0m [92mINF[0m request handled [2mrequest_id=[0m2658ebb4-2f24-490c-9093-289855f541eb [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m6.840125ms
[2m02:45:10[0m [92mINF[0m request handled [2mrequest_id=[0m8f7cf734-57a8-4427-9509-8434c3a46641 [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m7.163875ms
[2m02:45:10[0m [92mINF[0m request handled [2mrequest_id=[0m5bad7a4f-4e95-436d-ba76-a642f5a442de [2mmethod=[0mPOST [2mroute=[0m/properties/{id}/archive [2mstatus=[0m200 [2mduration=[0m6.090208ms
[2m02:45:46[0m [92mINF[0m fake sms sent [2mphone=[0m+79150380663 [2mmessage=[0m"rent 1500.00 ₽ запланировано на 17.06.2026"
[2m02:45:46[0m [92mINF[0m sms reminder sent [2mreminder_id=[0m38a9f21b-d053-4adb-bc4b-2bcc400ebf68 [2mevent_type=[0moperation_due
```
