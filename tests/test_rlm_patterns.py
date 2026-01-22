#!/usr/bin/env python3
"""
Comprehensive RLM Test Suite Based on Paper Patterns
https://alexzhang13.github.io/blog/2025/rlm/

Tests both Python and Go implementations for parity across all key RLM patterns:
1. Peeking - LM peeks at context structure
2. Grepping - LM uses regex to narrow search space
3. Partition + Map - LM chunks context and recursively processes
4. Summarization - LM summarizes subsets of context
5. Long-input, long-output - Git diff tracking, BibTeX generation
6. Multi-hop reasoning - Finding facts across context
7. Edge cases - Empty context, malformed data, large numbers
"""

import os
import sys
import json
import subprocess
import time
from typing import Dict, Any, Tuple
from datetime import datetime

# Add recursive-llm to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'recursive-llm', 'src'))

try:
    from rlm import RLM
    PYTHON_AVAILABLE = True
except ImportError:
    print("Warning: Python RLM not available")
    PYTHON_AVAILABLE = False

# Configuration
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY")
MODEL = "gpt-4o-mini"
GO_BINARY = os.path.join(os.path.dirname(__file__), "bin", "rlm-go")

class TestCase:
    def __init__(self, name: str, category: str, query: str, context: str, 
                 validator=None, expected_contains=None, expected_exact=None):
        self.name = name
        self.category = category
        self.query = query
        self.context = context
        self.validator = validator
        self.expected_contains = expected_contains or []
        self.expected_exact = expected_exact
        
    def validate(self, result: str) -> Tuple[bool, str]:
        """Validate result against expectations"""
        if self.validator:
            return self.validator(result)
        
        if self.expected_exact:
            if result.strip() == str(self.expected_exact).strip():
                return True, "Exact match"
            return False, f"Expected '{self.expected_exact}', got '{result}'"
        
        if self.expected_contains:
            for expected in self.expected_contains:
                if expected.lower() not in result.lower():
                    return False, f"Missing expected content: '{expected}'"
            return True, "All expected content found"
        
        return True, "No validation rules"

# ============================================================================
# TEST CASES - Based on RLM Paper Patterns
# ============================================================================

TEST_CASES = [
    # CATEGORY 1: PEEKING (Paper section on "Peeking")
    # LM should peek at first part of context to understand structure
    TestCase(
        name="peek_structured_list",
        category="peeking",
        query="What is the structure of this data? List the first 3 items.",
        context="\n".join([f"Item {i}: Value {i*10}" for i in range(1, 101)]),
        expected_contains=["Item 1", "Item 2", "Item 3"]
    ),
    
    TestCase(
        name="peek_json_structure",
        category="peeking",
        query="What fields are in each record? Give me the keys.",
        context=json.dumps([
            {"id": i, "name": f"User {i}", "score": i*10, "active": True}
            for i in range(1, 51)
        ]),
        expected_contains=["id", "name", "score", "active"]
    ),
    
    # CATEGORY 2: GREPPING (Paper section on "Grepping")
    # LM should use regex/search to find specific patterns
    TestCase(
        name="grep_email_addresses",
        category="grepping",
        query="Find all email addresses in the text.",
        context="""
        Contact info:
        John: john@example.com
        Random text here with no emails.
        More text and numbers: 12345
        Mary: mary@test.org
        Bob: bob@company.net
        """,
        validator=lambda r: (
            ("john@example.com" in r.lower() and 
             "mary@test.org" in r.lower() and 
             "bob@company.net" in r.lower()),
            "Should find all 3 emails"
        )
    ),
    
    TestCase(
        name="grep_question_lines",
        category="grepping",
        query="How many lines contain questions (end with '?')?",
        context="""
        This is a statement.
        Is this a question?
        Another statement here.
        What about this one?
        No question mark here
        Really, are you sure?
        Final statement.
        """,
        expected_exact="3"
    ),
    
    TestCase(
        name="grep_ids_pattern",
        category="grepping",
        query="Find all ID numbers (format: ID-####).",
        context="""
        Record ID-1234: Test data
        Some random text
        Record ID-5678: More data
        No ID here
        Record ID-9012: Final record
        """,
        validator=lambda r: (
            ("1234" in r and "5678" in r and "9012" in r),
            "Should find all 3 IDs"
        )
    ),
    
    # CATEGORY 3: PARTITION + MAP (Paper section on "Partition + Map")
    # LM should chunk and recursively process
    TestCase(
        name="partition_sentiment_analysis",
        category="partition_map",
        query="Count how many sections are positive vs negative in sentiment.",
        context="""
        Section 1: This is absolutely wonderful and amazing! Best day ever.
        Section 2: Terrible experience. Very disappointed and frustrated.
        Section 3: Great work! Excellent results and very happy with outcome.
        Section 4: Awful quality. Completely unsatisfied with everything.
        Section 5: Fantastic! Exceeded all expectations, truly impressive.
        """,
        validator=lambda r: (
            ("3" in r or "three" in r.lower()) and ("positive" in r.lower() or "negative" in r.lower()),
            "Should identify 3 positive and 2 negative"
        )
    ),
    
    TestCase(
        name="partition_category_labeling",
        category="partition_map",
        query="Categorize each paragraph as 'tech', 'sports', or 'food'.",
        context="""
        Paragraph 1: The new AI model uses transformer architecture with attention mechanisms.
        Paragraph 2: The championship game went into overtime with a final score of 28-24.
        Paragraph 3: This recipe requires butter, flour, and eggs for the perfect cake.
        Paragraph 4: Machine learning algorithms can process vast amounts of data efficiently.
        Paragraph 5: The quarterback threw three touchdowns in the first half.
        """,
        validator=lambda r: (
            ("tech" in r.lower() and "sports" in r.lower() and "food" in r.lower()),
            "Should identify all three categories"
        )
    ),
    
    # CATEGORY 4: SUMMARIZATION (Paper section on "Summarization")
    TestCase(
        name="summarize_sections",
        category="summarization",
        query="Summarize each section in one sentence.",
        context="""
        Section A: Introduction
        This paper discusses the benefits of recursive language models for processing long contexts.
        We introduce a novel framework that allows models to decompose and interact with context.
        The approach shows significant improvements over baseline methods.
        
        Section B: Methods
        Our method uses a REPL environment where context is stored as a variable.
        The model can peek, grep, partition, and recursively process the context.
        We implement this using Python notebooks and allow recursive LM calls.
        
        Section C: Results
        We achieve 33% improvement over GPT-5 baseline on OOLONG benchmark.
        The approach scales to 10M+ tokens without degradation.
        Cost per query is lower than direct approaches.
        """,
        validator=lambda r: (
            len(r) > 100 and ("section" in r.lower() or "summarize" in r.lower()),
            "Should provide summaries"
        )
    ),
    
    # CATEGORY 5: LONG-INPUT, LONG-OUTPUT (Paper section on git diff tracking)
    TestCase(
        name="git_diff_tracking_simple",
        category="long_output",
        query="What is the final state of shopping_list.txt after all commits?",
        context="""
Initial commit - shopping_list.txt:
apples
milk
bread
eggs
coffee

Commit 1 - Change apples to oranges:
-apples
+oranges

Commit 2 - Add cheese:
+cheese

Commit 3 - Remove milk:
-milk

Commit 4 - Add butter:
+butter
        """,
        validator=lambda r: (
            ("oranges" in r.lower() and "cheese" in r.lower() and 
             "butter" in r.lower() and "milk" not in r.lower()),
            "Final list should have oranges, bread, eggs, coffee, cheese, butter (no milk, no apples)"
        )
    ),
    
    TestCase(
        name="bibtex_generation",
        category="long_output",
        query="Generate BibTeX entries for these papers.",
        context="""
        Paper 1: "Attention Is All You Need" by Vaswani et al., published in NeurIPS 2017
        Paper 2: "BERT: Pre-training of Deep Bidirectional Transformers" by Devlin et al., NAACL 2019
        """,
        validator=lambda r: (
            ("@" in r and "title" in r.lower() and "author" in r.lower()),
            "Should generate BibTeX format"
        )
    ),
    
    # CATEGORY 6: MULTI-HOP REASONING
    TestCase(
        name="multi_hop_facts",
        category="multi_hop",
        query="Who is Alice's manager's manager?",
        context="""
        Alice reports to Bob.
        Bob reports to Carol.
        Carol reports to David.
        David is the CEO.
        
        Bob manages the engineering team.
        Carol manages all of technology.
        """,
        expected_exact="Carol"
    ),
    
    TestCase(
        name="multi_hop_calculations",
        category="multi_hop",
        query="What is the total price for all items Alice bought?",
        context="""
        Alice bought: ItemA, ItemB, ItemC
        Bob bought: ItemD, ItemE
        
        Prices:
        ItemA: $10
        ItemB: $25
        ItemC: $15
        ItemD: $30
        ItemE: $20
        """,
        expected_exact="50"
    ),
    
    # CATEGORY 7: EDGE CASES
    TestCase(
        name="edge_empty_result",
        category="edge_cases",
        query="Find all phone numbers.",
        context="This text contains no phone numbers at all. Just random words and sentences.",
        validator=lambda r: (
            ("none" in r.lower() or "no" in r.lower() or "0" in r or "not found" in r.lower()),
            "Should indicate no phone numbers found"
        )
    ),
    
    TestCase(
        name="edge_large_numbers",
        category="edge_cases",
        query="What is the sum of all numbers?",
        context="Numbers: 1234567, 9876543, 5555555, 7777777",
        expected_exact="24444442"
    ),
    
    TestCase(
        name="edge_nested_structure",
        category="edge_cases",
        query="How many total people are mentioned across all departments?",
        context="""
        Engineering Department:
        - Team A: Alice, Bob, Carol
        - Team B: David, Eve
        
        Marketing Department:
        - Team X: Frank, Grace
        - Team Y: Henry
        
        Sales Department:
        - Team Z: Ivy, Jack, Kate, Leo
        """,
        expected_exact="11"
    ),
    
    # CATEGORY 8: CONTEXT ROT TESTS (should not degrade with long context)
    TestCase(
        name="context_rot_needle",
        category="context_rot",
        query="What is the secret code?",
        context="\n".join([
            "Random filler text line number " + str(i) + " with various words."
            if i != 500 else "The secret code is: ALPHA-BRAVO-CHARLIE"
            for i in range(1000)
        ]),
        expected_contains=["ALPHA-BRAVO-CHARLIE"]
    ),
    
    TestCase(
        name="context_rot_last_item",
        category="context_rot",
        query="What is the last item number?",
        context="\n".join([f"Item {i}: Data for item {i}" for i in range(1, 501)]),
        expected_exact="500"
    ),
]

# ============================================================================
# TEST RUNNERS
# ============================================================================

def run_python_test(test: TestCase) -> Dict[str, Any]:
    """Run test with Python RLM implementation"""
    if not PYTHON_AVAILABLE:
        return {"error": "Python RLM not available"}
    
    try:
        rlm = RLM(model=MODEL, api_key=OPENAI_API_KEY, max_iterations=30)
        start = time.time()
        result = rlm.completion(query=test.query, context=test.context)
        duration = time.time() - start
        
        # Handle both dict and string results
        if isinstance(result, str):
            return {
                "result": result,
                "stats": {},
                "duration": duration
            }
        
        return {
            "result": result.get("result", result.get("answer", "")),
            "stats": result.get("stats", {}),
            "duration": duration
        }
    except Exception as e:
        return {"error": str(e)}

def run_go_test(test: TestCase) -> Dict[str, Any]:
    """Run test with Go RLM implementation"""
    if not os.path.exists(GO_BINARY):
        return {"error": f"Go binary not found at {GO_BINARY}"}
    
    try:
        input_data = {
            "model": MODEL,
            "query": test.query,
            "context": test.context,
            "config": {
                "api_key": OPENAI_API_KEY,
                "max_iterations": 30
            }
        }
        
        start = time.time()
        result = subprocess.run(
            [GO_BINARY],
            input=json.dumps(input_data),
            capture_output=True,
            text=True,
            timeout=300  # 5 minutes for complex/long queries
        )
        duration = time.time() - start
        
        if result.returncode != 0:
            return {"error": f"Go binary failed: {result.stderr}"}
        
        output = json.loads(result.stdout)
        return {
            "result": output.get("result", ""),
            "stats": output.get("stats", {}),
            "duration": duration
        }
    except Exception as e:
        return {"error": str(e)}

# ============================================================================
# MAIN TEST EXECUTION
# ============================================================================

def run_all_tests(impl: str = "both"):
    """Run all tests for specified implementation(s)"""
    
    results = {
        "timestamp": datetime.now().isoformat(),
        "model": MODEL,
        "tests": []
    }
    
    print(f"\n{'='*80}")
    print(f"RLM PATTERN-BASED TEST SUITE")
    print(f"Based on: https://alexzhang13.github.io/blog/2025/rlm/")
    print(f"{'='*80}\n")
    
    for i, test in enumerate(TEST_CASES, 1):
        print(f"\n[{i}/{len(TEST_CASES)}] {test.name} ({test.category})")
        print(f"Query: {test.query[:80]}...")
        print(f"Context: {len(test.context)} chars")
        
        test_result = {
            "name": test.name,
            "category": test.category,
            "query": test.query,
            "context_length": len(test.context)
        }
        
        # Run Python test
        if impl in ["python", "both"] and PYTHON_AVAILABLE:
            print("  Running Python...", end=" ", flush=True)
            py_result = run_python_test(test)
            
            if "error" in py_result:
                print(f"❌ ERROR: {py_result['error']}")
                test_result["python"] = {"error": py_result["error"]}
            else:
                valid, msg = test.validate(py_result["result"])
                status = "✅" if valid else "❌"
                print(f"{status} {py_result['duration']:.2f}s - {msg}")
                print(f"    Result: {py_result['result'][:100]}")
                print(f"    Stats: {py_result['stats']}")
                
                test_result["python"] = {
                    "result": py_result["result"],
                    "stats": py_result["stats"],
                    "duration": py_result["duration"],
                    "valid": valid,
                    "validation_message": msg
                }
        
        # Run Go test
        if impl in ["go", "both"]:
            print("  Running Go...", end=" ", flush=True)
            go_result = run_go_test(test)
            
            if "error" in go_result:
                print(f"❌ ERROR: {go_result['error']}")
                test_result["go"] = {"error": go_result["error"]}
            else:
                valid, msg = test.validate(go_result["result"])
                status = "✅" if valid else "❌"
                print(f"{status} {go_result['duration']:.2f}s - {msg}")
                print(f"    Result: {go_result['result'][:100]}")
                print(f"    Stats: {go_result['stats']}")
                
                test_result["go"] = {
                    "result": go_result["result"],
                    "stats": go_result["stats"],
                    "duration": go_result["duration"],
                    "valid": valid,
                    "validation_message": msg
                }
        
        # Compare results if both ran
        if "python" in test_result and "go" in test_result:
            if "error" not in test_result["python"] and "error" not in test_result["go"]:
                py_valid = test_result["python"]["valid"]
                go_valid = test_result["go"]["valid"]
                
                if py_valid and go_valid:
                    test_result["parity"] = "✅ BOTH PASS"
                elif not py_valid and not go_valid:
                    test_result["parity"] = "⚠️  BOTH FAIL"
                elif py_valid:
                    test_result["parity"] = "❌ PYTHON ONLY"
                else:
                    test_result["parity"] = "❌ GO ONLY"
                
                print(f"  Parity: {test_result['parity']}")
        
        results["tests"].append(test_result)
    
    # Summary
    print(f"\n{'='*80}")
    print("SUMMARY BY CATEGORY")
    print(f"{'='*80}\n")
    
    categories = {}
    for test in results["tests"]:
        cat = test["category"]
        if cat not in categories:
            categories[cat] = {"total": 0, "py_pass": 0, "go_pass": 0, "both_pass": 0}
        
        categories[cat]["total"] += 1
        
        if "python" in test and test["python"].get("valid"):
            categories[cat]["py_pass"] += 1
        if "go" in test and test["go"].get("valid"):
            categories[cat]["go_pass"] += 1
        if (test.get("python", {}).get("valid") and test.get("go", {}).get("valid")):
            categories[cat]["both_pass"] += 1
    
    for cat, stats in sorted(categories.items()):
        print(f"{cat:20} | Total: {stats['total']:2} | "
              f"Python: {stats['py_pass']:2}/{stats['total']:2} | "
              f"Go: {stats['go_pass']:2}/{stats['total']:2} | "
              f"Both: {stats['both_pass']:2}/{stats['total']:2}")
    
    # Save results
    output_file = f"test_results_{impl}_{datetime.now().strftime('%Y%m%d_%H%M%S')}.json"
    with open(output_file, 'w') as f:
        json.dump(results, f, indent=2)
    
    print(f"\n✅ Results saved to: {output_file}\n")
    
    return results

if __name__ == "__main__":
    import argparse
    
    parser = argparse.ArgumentParser(description="Run RLM pattern-based tests")
    parser.add_argument("--impl", choices=["python", "go", "both"], default="both",
                        help="Which implementation to test")
    parser.add_argument("--category", help="Only run tests from specific category")
    parser.add_argument("--test", help="Only run specific test by name")
    
    args = parser.parse_args()
    
    # Filter tests if requested
    if args.category:
        TEST_CASES = [t for t in TEST_CASES if t.category == args.category]
        print(f"Running only '{args.category}' tests ({len(TEST_CASES)} tests)")
    
    if args.test:
        TEST_CASES = [t for t in TEST_CASES if t.name == args.test]
        print(f"Running only '{args.test}' test")
    
    if not TEST_CASES:
        print("No tests to run!")
        sys.exit(1)
    
    run_all_tests(impl=args.impl)
