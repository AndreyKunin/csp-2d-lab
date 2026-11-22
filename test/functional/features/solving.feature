Feature: Solving known instances
  Solution invariants (checked by the independent oracle) and known optima.

  Background:
    Given the server is running with config "fast"

  @FT-SOLVE-01
  Scenario: A perfect tiling uses one sheet without waste
    Given a running simulation for fixture "D04-four-identical" with maxEpochsCount 200
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 1 sheet
    And the best solution waste is 0

  @FT-SOLVE-02
  Scenario: A perfect tiling that accounts for the saw width
    Given a running simulation for fixture "D05-tiling-kerf-ok" with maxEpochsCount 200
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 1 sheet

  @FT-SOLVE-03
  Scenario: Saw width prevents a perfect tiling
    Given a running simulation for fixture "D06-kerf-breaks" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 4 sheets

  @FT-SOLVE-04
  Scenario: Identical parts over several sheets
    Given a running simulation for fixture "D07-identical-multi" with maxEpochsCount 200
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 3 sheets

  # D08: 4 identical parts of exactly the sheet size (40 distinct genotypes; the
  # small-input overrides make the population 16).
  @FT-SOLVE-05
  Scenario: Parts of exactly the sheet size
    Given a running simulation for fixture "D08-exact-sheet" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 4 sheets
    And every sheet of the best solution holds a single part

  # D09: 4 identical parts that fit the sheet only rotated, one per sheet.
  @FT-SOLVE-06
  Scenario: Parts that fit only rotated
    Given a running simulation for fixture "D09-rotation" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid
    And the best solution uses 4 sheets

  @FT-SOLVE-07
  Scenario: Unconstrained layout
    Given a running simulation for fixture "D11-unconstrained" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  @FT-SOLVE-08
  Scenario: Fractional dimensions and saw width
    Given a running simulation for fixture "D12-fractional" with maxEpochsCount 50
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And every solution is valid

  # D13 lower bound: 50 800 / 29 768 -> 2 sheets; LB + 1 = 3.
  @FT-SOLVE-09 @quality
  Scenario: A mixed instance gets within one sheet of the lower bound
    Given the server is running with config "quality"
    And a start request from fixture "D13-mixed-20"
    Then at least 4 of 5 runs reach 3 sheets within 3000 epochs

  # The optimal layout stacks 60x50 on 50x40 into a 50x100 column, which needs
  # a rotation on an exact tie (s1 == l2 == 50). Regression test for the former
  # GA-12: with a strict ">" tie-break the optimum could not be encoded (S gave
  # 110x50, L 60x90) and 5 of 5 runs ended at 3 sheets.
  @FT-SOLVE-10 @quality
  Scenario: A guillotine-perfect instance reaches its optimum
    Given the server is running with config "quality"
    And a start request from fixture "D16-guillotine-opt"
    Then at least 4 of 5 runs reach 2 sheets within 3000 epochs

  # D04 has only a few optimal genotypes, so 2 elites can converge to equal fitness.
  @FT-SOLVE-11
  Scenario: The early stop ends a converged run before the epoch limit
    Given a running simulation for fixture "D04-four-identical" with parameters:
      | eliteCount     | 2      |
      | windowSize     | 10     |
      | maxEpochsCount | 100000 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the epoch number is at most 99999

  # A former early stop latched on the elite deviation, which is always 0 with
  # one elite, and ended the run at epoch windowSize. D04 has 4 parts, so the
  # elite count is 1 (small-input override); the run must go through all its
  # cycles and the championship (the "fast" profile: 10 cycles).
  @FT-SOLVE-12
  Scenario: A one-elite run is not cut at epoch windowSize
    Given a running simulation for fixture "D04-four-identical" with parameters:
      | windowSize     | 10     |
      | maxEpochsCount | 100000 |
    When I wait for job completion within 60 seconds
    Then the job status is "completed"
    And the reported parameter "eliteCount" is 1
    And the run finished 10 cycles
    And the championship has started

  @FT-SOLVE-13
  Scenario: The best fitness never gets worse during a run
    Given a running simulation for fixture "D13-mixed-20" with maxEpochsCount 150
    When I wait for job completion within 120 seconds
    Then the job status is "completed"
    And the best fitness never increases
    And the epoch numbers never decrease

  @FT-SOLVE-14 @slow
  Scenario: A medium instance produces valid layouts
    Given a running simulation for fixture "D14-medium-60" with maxEpochsCount 300
    When I wait for job completion within 300 seconds
    Then the job status is "completed"
    And every solution is valid
