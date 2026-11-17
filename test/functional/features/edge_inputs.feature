Feature: Small and degenerate problems
  Edge inputs. Scenarios that probe the population size use
  the production bounds (100..60000) explicitly.

  Background:
    Given the server is running with config "fast"

  @FT-EDGE-01 @pending
  Scenario: A single part
    Given a running simulation for fixture "D01-single" with parameters:
      | populationSize.min | 100   |
      | populationSize.max | 60000 |
      | maxEpochsCount     | 50    |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 1 sheet

  @FT-EDGE-02 @pending
  Scenario: Two parts
    Given a running simulation for fixture "D02-pair" with parameters:
      | populationSize.min | 100   |
      | populationSize.max | 60000 |
      | maxEpochsCount     | 50    |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the best solution uses 1 sheet

  @FT-EDGE-03 @pending
  Scenario: Three distinct parts
    Given a running simulation for fixture "D03-three-distinct" with parameters:
      | populationSize.min | 100   |
      | populationSize.max | 60000 |
      | maxEpochsCount     | 50    |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the best solution uses 1 sheet

  @FT-EDGE-04 @pending
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

  # Three identical parts have only 8 distinct genotypes, fewer than the
  # minimum population, so the population cannot be built today.
  @FT-EDGE-08 @pending
  Scenario: Parts exactly the sheet size with a saw width
    Given a running simulation for fixture "E08-exact-sheet-kerf" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 3 sheets
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
