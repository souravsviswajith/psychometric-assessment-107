/**
 * api/types.ts — the shapes the Go runtime serves.
 *
 * These mirror the JSON produced by the Go domain package. The UI only reads
 * them: validation, identity, and every invariant live on the Go side, so the
 * UI cannot drift into being the model (UI ≠ Truth, §14).
 */

export interface ActorRef {
  readonly kind: 'user' | 'system' | 'capability' | 'model';
  readonly id: string;
  readonly label?: string;
}

export interface ConstructRef {
  readonly constructId: string;
  readonly label: string;
  readonly kind: string;
  readonly taxonomyRef: string;
  readonly taxonomyVersion: string;
}

export interface InterpretationPolicy {
  readonly id: string;
  readonly version: string;
  readonly direction: 'ascending' | 'descending';
  readonly statement: string;
}

export interface ResponseInterpretation {
  readonly kind: 'ordinal' | 'categorical' | 'text';
  readonly shape: string;
  readonly policy: InterpretationPolicy;
  readonly rawIndex?: number;
  readonly presentedLabels?: readonly string[];
  readonly position?: number;
  readonly rawValue?: number;
  readonly selectedOptionIds?: readonly string[];
  readonly selectedLabels?: readonly string[];
  readonly text?: string;
  readonly charCount?: number;
}

export interface InstrumentRef {
  readonly instrumentId: string;
  readonly instrumentVersion: string;
  readonly itemRef: string;
  readonly itemVersion: string;
  readonly promptText: string;
  readonly responseShape: string;
  readonly validated: boolean;
  readonly validationRef?: string;
}

export interface InstrumentOption {
  readonly id: string;
  readonly label: string;
}

export interface InstrumentItem {
  readonly instrumentId: string;
  readonly instrumentVersion: string;
  readonly itemRef: string;
  readonly itemVersion: string;
  readonly promptText: string;
  readonly responseShape: string;
  readonly options: readonly InstrumentOption[];
  readonly constructs: readonly ConstructRef[];
  readonly validated: boolean;
  readonly limitations: readonly string[];
}

export interface EvidenceRecord {
  readonly id: string;
  readonly signalId: string;
  readonly status: 'primary' | 'derived';
  readonly provenance: {
    readonly sourceRef: string;
    readonly sourceKind: string;
    readonly assertedBy: ActorRef;
    readonly capturedBy: ActorRef;
    readonly lineage: readonly string[];
  };
  readonly method: {
    readonly ref: string;
    readonly kind: string;
    readonly version: string;
    readonly description: string;
    readonly limitations: readonly string[];
  };
  readonly temporal: { readonly observedAt: string; readonly recordedAt: string; readonly sessionRef?: string };
  readonly quality: {
    readonly directness: string;
    readonly recency: string;
    readonly completeness: string;
    readonly reliability: string;
    readonly validity: string;
    readonly sampleSize: number;
    readonly notes?: string;
  };
}

export type ContextValue = string | number | boolean | readonly string[];

export interface ContextField {
  readonly key: string;
  readonly value: ContextValue;
  readonly assertedBy: ActorRef;
}

export interface CareerContext {
  readonly id: string;
  readonly kind: string;
  readonly fields: readonly ContextField[];
}

export interface UncertaintyNote {
  readonly id: string;
  readonly code: string;
  readonly source: string;
  readonly kind: string;
  readonly level: 'low' | 'moderate' | 'high' | 'unknown';
  readonly detail: string;
  readonly affects: readonly string[];
  readonly mitigation?: string;
  readonly notedBy: ActorRef;
}

export interface UncertaintyProfile {
  readonly profileVersion: string;
  readonly coverage: string;
  readonly notes: readonly UncertaintyNote[];
}

export interface Signal {
  readonly id: string;
  readonly subjectId: string;
  readonly instrument: InstrumentRef;
  readonly response: ResponseInterpretation;
  readonly capturedAt: string;
  readonly context: CareerContext;
  readonly evidence: readonly EvidenceRecord[];
  readonly uncertainty: UncertaintyProfile;
}

export interface Representation1to3 {
  readonly id: string;
  readonly schemaVersion: string;
  readonly signalId: string;
  readonly subjectId: string;
  readonly A: {
    readonly id: string;
    readonly instrument: InstrumentRef;
    readonly constructs: readonly ConstructRef[];
    readonly response: ResponseInterpretation;
    readonly observedAt: string;
    readonly setting: string;
  };
  readonly E: { readonly id: string; readonly primaryEvidenceId: string; readonly bundle: readonly EvidenceRecord[] };
  readonly C: CareerContext;
  readonly U: UncertaintyProfile;
  readonly createdAt: string;
  readonly createdBy: ActorRef;
  readonly transformationRef: string;
}

export interface RenderedRow {
  readonly label: string;
  readonly value: string;
  readonly detail?: string;
  readonly provenance?: string;
}

export interface RenderedRepresentation {
  readonly headline: string;
  readonly subhead: string;
  readonly sections: readonly { readonly key: 'A' | 'E' | 'C' | 'U'; readonly title: string; readonly rows: readonly RenderedRow[] }[];
}

export type RequirementStatus = 'supported' | 'conflicting' | 'gap' | 'undetermined';

export interface PathwayRequirement {
  readonly requirementId: string;
  readonly constructId: string;
  readonly statement: string;
  readonly kind: string;
  readonly status: RequirementStatus;
  readonly observations: readonly { readonly value: string; readonly evidenceRefs: readonly string[] }[];
  readonly evidenceRefs: readonly string[];
  readonly rationale: string;
}

export type ReviewState = 'unreviewed' | 'interesting' | 'not-a-fit' | 'pursuing' | 'deferred';

export interface PathwayCandidate {
  readonly id: string;
  readonly pathwayId: string;
  readonly name: string;
  readonly family: string;
  readonly description: string;
  readonly requirements: readonly PathwayRequirement[];
  readonly supportSummary: { readonly supported: number; readonly conflicting: number; readonly gap: number; readonly undetermined: number; readonly total: number };
  readonly entryRoutes: readonly string[];
  readonly tradeoffs: readonly { readonly statement: string; readonly costsWhat: string }[];
  readonly openQuestions: readonly string[];
  readonly uncertainty: readonly UncertaintyNote[];
  readonly supportingRepresentationRefs: readonly string[];
  readonly supportingTransformationRefs: readonly string[];
  readonly evidenceRefs: readonly string[];
  readonly sourceRef: string;
  readonly review: { readonly state: ReviewState; readonly decidedBy: ActorRef; readonly decidedAt?: string };
}

export interface PathwaySet {
  readonly id: string;
  readonly schemaVersion: string;
  readonly createdAt: string;
  readonly pathways: readonly PathwayCandidate[];
  readonly coverage: {
    readonly assessedConstructs: readonly string[];
    readonly unassessedConstructs: readonly string[];
    readonly observationCount: number;
    readonly representationCount: number;
  };
  readonly mappingRef: string;
  readonly mappingPolicyRef: string;
  readonly restrictedUse: readonly string[];
}

export interface DecisionSummary {
  readonly total: number;
  readonly unreviewed: number;
  readonly explored: number;
  readonly declined: number;
  readonly pursuing: number;
}

export interface UserDecisionRecord {
  readonly id: string;
  readonly decisionKind: string;
  readonly targetRef: string;
  readonly decidedAt: string;
}

export interface CorrectionRecord {
  readonly id: string;
  readonly targetRef: string;
  readonly dimension: string;
  readonly previousValueSummary: string;
  readonly correctedValueSummary: string;
  readonly reason: string;
}

export interface ProposalRecord {
  readonly id: string;
  readonly proposalKind: string;
  readonly modelMetadata: {
    readonly provider: string;
    readonly model: string;
    readonly modelVersion: string;
    readonly instructionVersion: string;
    readonly uncertainty: string;
  };
  readonly summary: string;
  readonly evidenceRefs: readonly string[];
  readonly uncertainty: readonly UncertaintyNote[];
  readonly status: string;
}

export interface RuntimeProjection {
  readonly subjectId: string;
  readonly signalCount: number;
  readonly representationCount: number;
  readonly pathwaySetCount: number;
  readonly transformationCount: number;
  readonly executionCount: number;
  readonly decisionCount: number;
  readonly proposalCount: number;
  readonly uncertaintyNotes: number;
  readonly contractVersions: Readonly<Record<string, string>>;
  readonly lastActivityAt: string;
}

export interface SessionView {
  readonly projection: RuntimeProjection;
  readonly signals: readonly Signal[];
  readonly representations: readonly Representation1to3[];
  readonly rendered: Readonly<Record<string, RenderedRepresentation>>;
  readonly pathwaySets: readonly PathwaySet[];
  readonly decisionSummaries: Readonly<Record<string, DecisionSummary>>;
  readonly decisions: readonly UserDecisionRecord[];
  readonly corrections: readonly CorrectionRecord[];
  readonly proposals: readonly ProposalRecord[];
  readonly persisted: boolean;
}

export interface CapabilityContract {
  readonly id: string;
  readonly requiredScopes: readonly string[];
  readonly requiresModel: boolean;
  readonly transformationRef?: string;
  readonly userBoundary: string;
}

export interface InvariantDescriptor {
  readonly id: string;
  readonly statement: string;
  readonly scope: string;
  readonly mandatory: boolean;
}

export interface Catalog {
  readonly items: readonly InstrumentItem[];
  readonly capabilityContracts: readonly CapabilityContract[];
  readonly contractVersions: readonly { readonly name: string; readonly version: string }[];
  readonly invariantDescriptors: readonly InvariantDescriptor[];
  readonly uncertaintyCodes: readonly string[];
  readonly forbiddenScoreKeys: readonly string[];
  readonly vocabularyId: string;
  readonly userAuthorityStatement: string;
  readonly noScalarCollapseStatement: string;
  readonly subjectId: string;
  readonly subjectLabel: string;
  readonly runtimeId: string;
}

export interface ExperimentMeasure {
  readonly label: string;
  readonly value: number | string | boolean;
  readonly unit?: string;
  readonly note?: string;
}

export interface ExperimentResult {
  readonly id: string;
  readonly title: string;
  readonly hypothesis: string;
  readonly method: string;
  readonly measures: readonly ExperimentMeasure[];
  readonly observations: readonly string[];
  readonly limitations: readonly string[];
  readonly status: string;
  readonly ranAt: string;
  readonly fixtureRef: string;
}

export interface ResearchRun {
  readonly results: readonly ExperimentResult[];
  readonly reproducibility: { readonly reproducible: boolean; readonly details: readonly string[] };
  readonly report: string;
}

/** What the capture screen sends; the server adds the subject and input identity. */
export interface CaptureInput {
  readonly instrumentId: string;
  readonly itemRef: string;
  readonly response: { readonly kind: 'option'; readonly optionId: string } | { readonly kind: 'text'; readonly text: string };
  readonly declaredContext: readonly { readonly key: string; readonly value: ContextValue }[];
  readonly source: { readonly kind: string; readonly ref: string; readonly label: string };
  readonly sessionRef?: string;
}

export interface ActionResponse {
  readonly notice?: string;
  readonly rejections: readonly string[];
  readonly view: SessionView;
}
