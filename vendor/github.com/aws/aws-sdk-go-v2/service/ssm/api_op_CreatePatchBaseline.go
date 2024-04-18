// Code generated by smithy-go-codegen DO NOT EDIT.

package ssm

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Creates a patch baseline. For information about valid key-value pairs in
// PatchFilters for each supported operating system type, see PatchFilter .
func (c *Client) CreatePatchBaseline(ctx context.Context, params *CreatePatchBaselineInput, optFns ...func(*Options)) (*CreatePatchBaselineOutput, error) {
	if params == nil {
		params = &CreatePatchBaselineInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "CreatePatchBaseline", params, optFns, c.addOperationCreatePatchBaselineMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*CreatePatchBaselineOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type CreatePatchBaselineInput struct {

	// The name of the patch baseline.
	//
	// This member is required.
	Name *string

	// A set of rules used to include patches in the baseline.
	ApprovalRules *types.PatchRuleGroup

	// A list of explicitly approved patches for the baseline. For information about
	// accepted formats for lists of approved patches and rejected patches, see About
	// package name formats for approved and rejected patch lists (https://docs.aws.amazon.com/systems-manager/latest/userguide/patch-manager-approved-rejected-package-name-formats.html)
	// in the Amazon Web Services Systems Manager User Guide.
	ApprovedPatches []string

	// Defines the compliance level for approved patches. When an approved patch is
	// reported as missing, this value describes the severity of the compliance
	// violation. The default value is UNSPECIFIED .
	ApprovedPatchesComplianceLevel types.PatchComplianceLevel

	// Indicates whether the list of approved patches includes non-security updates
	// that should be applied to the managed nodes. The default value is false .
	// Applies to Linux managed nodes only.
	ApprovedPatchesEnableNonSecurity *bool

	// User-provided idempotency token.
	ClientToken *string

	// A description of the patch baseline.
	Description *string

	// A set of global filters used to include patches in the baseline.
	GlobalFilters *types.PatchFilterGroup

	// Defines the operating system the patch baseline applies to. The default value
	// is WINDOWS .
	OperatingSystem types.OperatingSystem

	// A list of explicitly rejected patches for the baseline. For information about
	// accepted formats for lists of approved patches and rejected patches, see About
	// package name formats for approved and rejected patch lists (https://docs.aws.amazon.com/systems-manager/latest/userguide/patch-manager-approved-rejected-package-name-formats.html)
	// in the Amazon Web Services Systems Manager User Guide.
	RejectedPatches []string

	// The action for Patch Manager to take on patches included in the RejectedPackages
	// list.
	//   - ALLOW_AS_DEPENDENCY : A package in the Rejected patches list is installed
	//   only if it is a dependency of another package. It is considered compliant with
	//   the patch baseline, and its status is reported as InstalledOther . This is the
	//   default action if no option is specified.
	//   - BLOCK: Packages in the Rejected patches list, and packages that include
	//   them as dependencies, aren't installed by Patch Manager under any circumstances.
	//   If a package was installed before it was added to the Rejected patches list, or
	//   is installed outside of Patch Manager afterward, it's considered noncompliant
	//   with the patch baseline and its status is reported as InstalledRejected.
	RejectedPatchesAction types.PatchAction

	// Information about the patches to use to update the managed nodes, including
	// target operating systems and source repositories. Applies to Linux managed nodes
	// only.
	Sources []types.PatchSource

	// Optional metadata that you assign to a resource. Tags enable you to categorize
	// a resource in different ways, such as by purpose, owner, or environment. For
	// example, you might want to tag a patch baseline to identify the severity level
	// of patches it specifies and the operating system family it applies to. In this
	// case, you could specify the following key-value pairs:
	//   - Key=PatchSeverity,Value=Critical
	//   - Key=OS,Value=Windows
	// To add tags to an existing patch baseline, use the AddTagsToResource operation.
	Tags []types.Tag

	noSmithyDocumentSerde
}

type CreatePatchBaselineOutput struct {

	// The ID of the created patch baseline.
	BaselineId *string

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationCreatePatchBaselineMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsjson11_serializeOpCreatePatchBaseline{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsjson11_deserializeOpCreatePatchBaseline{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "CreatePatchBaseline"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = addClientRequestID(stack); err != nil {
		return err
	}
	if err = addComputeContentLength(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = addComputePayloadSHA256(stack); err != nil {
		return err
	}
	if err = addRetry(stack, options); err != nil {
		return err
	}
	if err = addRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = addRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addIdempotencyToken_opCreatePatchBaselineMiddleware(stack, options); err != nil {
		return err
	}
	if err = addOpCreatePatchBaselineValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opCreatePatchBaseline(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addRecursionDetection(stack); err != nil {
		return err
	}
	if err = addRequestIDRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	return nil
}

type idempotencyToken_initializeOpCreatePatchBaseline struct {
	tokenProvider IdempotencyTokenProvider
}

func (*idempotencyToken_initializeOpCreatePatchBaseline) ID() string {
	return "OperationIdempotencyTokenAutoFill"
}

func (m *idempotencyToken_initializeOpCreatePatchBaseline) HandleInitialize(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (
	out middleware.InitializeOutput, metadata middleware.Metadata, err error,
) {
	if m.tokenProvider == nil {
		return next.HandleInitialize(ctx, in)
	}

	input, ok := in.Parameters.(*CreatePatchBaselineInput)
	if !ok {
		return out, metadata, fmt.Errorf("expected middleware input to be of type *CreatePatchBaselineInput ")
	}

	if input.ClientToken == nil {
		t, err := m.tokenProvider.GetIdempotencyToken()
		if err != nil {
			return out, metadata, err
		}
		input.ClientToken = &t
	}
	return next.HandleInitialize(ctx, in)
}
func addIdempotencyToken_opCreatePatchBaselineMiddleware(stack *middleware.Stack, cfg Options) error {
	return stack.Initialize.Add(&idempotencyToken_initializeOpCreatePatchBaseline{tokenProvider: cfg.IdempotencyTokenProvider}, middleware.Before)
}

func newServiceMetadataMiddleware_opCreatePatchBaseline(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "CreatePatchBaseline",
	}
}