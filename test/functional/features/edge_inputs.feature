Feature: Small and degenerate problems
  Edge inputs. Scenarios that probe the population size use
  the production bounds (100..60000) explicitly.

  Background:
    Given the server is running with config "fast"

  # Fewer than 4 parts have too few distinct layouts for the GA (GA-05): the
  # server rejects them, like the UI (Model.validate).
  @FT-EDGE-01
  Scenario: A single part is rejected
    Given a start request from fixture "D01-single"
    When I start the simulation
    Then the response status is 400
    And the response message contains "at least 4 parts"

  @FT-EDGE-02
  Scenario: Two parts are rejected
    Given a start request from fixture "D02-pair"
    When I start the simulation
    Then the response status is 400
    And the response message contains "at least 4 parts"

  @FT-EDGE-03
  Scenario: Three distinct parts are rejected
    Given a start request from fixture "D03-three-distinct"
    When I start the simulation
    Then the response status is 400
    And the response message contains "at least 4 parts"

  @FT-EDGE-04
  Scenario: Four identical parts with the default population
    Given a running simulation for fixture "D04-four-identical" with parameters:
      | populationSize.min | 100   |
      | populationSize.max | 60000 |
      | maxEpochsCount     | 50    |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  @FT-EDGE-05
  Scenario: Five identical parts with the default population
    Given a running simulation for fixture "E05-five-identical" with parameters:
      | populationSize.min | 100   |
      | populationSize.max | 60000 |
      | maxEpochsCount     | 50    |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  @FT-EDGE-06
  Scenario: Many labels with one part each
    Given a running simulation for fixture "D17-many-labels" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  @FT-EDGE-07 @pending
  Scenario: The same label with different sizes is rejected
    Given a start request from fixture "X11-duplicate-labels"
    When I start the simulation
    Then the response status is 400

  # E08: 4 identical parts of exactly the sheet size, with a saw width.
  @FT-EDGE-08
  Scenario: Parts exactly the sheet size with a saw width
    Given a running simulation for fixture "E08-exact-sheet-kerf" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 4 sheets
    And every sheet of the best solution holds a single part

  @FT-EDGE-09
  Scenario: Parts exactly half the sheet
    Given a running simulation for fixture "E09-half-sheet" with maxEpochsCount 100
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 2 sheets

  @FT-EDGE-10 @quality
  Scenario: Very thin parts
    Given the server is running with config "quality"
    And a running simulation for fixture "E10-thin-parts" with maxEpochsCount 3000
    When I wait for job completion within 600 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 1 sheet

  @FT-EDGE-11
  Scenario: A sheet given with length shorter than width
    Given a running simulation for fixture "E11-sheet-swapped" with maxEpochsCount 200
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 3 sheets
