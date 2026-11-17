Feature: PDF report
  The PDF report endpoint.

  Background:
    Given the server is running with config "fast"

  @FT-PDF-01
  Scenario: A report with images
    When I request a PDF for fixture "D13-mixed-20" with 2 images
    Then the response is a PDF document
    And the response header "Content-Disposition" contains "inline"

  @FT-PDF-02
  Scenario: A report without images
    When I request a PDF for fixture "D13-mixed-20" with 0 images
    Then the response is a PDF document

  @FT-PDF-03
  Scenario: A report for an unconstrained problem without images
    When I request a PDF for fixture "D11-unconstrained" with 0 images
    Then the response is a PDF document

  @FT-PDF-04
  Scenario: The request field is required
    When I request a PDF without the request field
    Then the response status is 400
    And the error message contains "missing 'request' form field"

  @FT-PDF-05
  Scenario: The request field must be JSON
    When I request a PDF with request field '{not json'
    Then the response status is 400
    And the error message contains "invalid request JSON"

  @FT-PDF-06
  Scenario: The body must be multipart
    When I send a non-multipart PDF request
    Then the response status is 400

  @FT-PDF-07
  Scenario: Bodies over 32 MB are refused
    When I request a PDF for fixture "D13-mixed-20" with a 33 MB image
    Then the response arrives within 10 seconds
    And the response status is one of 400, 413
    And the server is healthy

  @FT-PDF-08
  Scenario: An image that is not a PNG
    When I request a PDF for fixture "D13-mixed-20" with an invalid image
    Then the response is an error with a message
    And the server is healthy

  @FT-PDF-09
  Scenario: The report contains its sections
    When I request a PDF for fixture "D13-mixed-20" with 1 image
    Then the PDF contains the text "Layout Report"
    And the PDF contains the text "Saw width"
