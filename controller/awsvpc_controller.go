/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1 "github.com/SMJNayeem/aws-vpc-operator/api/v1"
)

type AWSVPCReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *AWSVPCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	awsvpc := &infrastructurev1.AWSVPC{}
	err := r.Get(ctx, req.NamespacedName, awsvpc)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(awsvpc.Spec.Region),
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	svc := ec2.New(sess)

	if awsvpc.Status.VPCID == "" {
		// Create new VPC
		result, err := r.createVPC(svc, awsvpc)
		if err != nil {
			return result, err
		}
	} else {
		// Update existing VPC
		result, err := r.updateVPC(svc, awsvpc)
		if err != nil {
			return result, err
		}
	}

	if err := r.Status().Update(ctx, awsvpc); err != nil {
		log.Error(err, "failed to update AWSVPC status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AWSVPCReconciler) createVPC(svc *ec2.EC2, awsvpc *infrastructurev1.AWSVPC) (ctrl.Result, error) {
	createVpcInput := &ec2.CreateVpcInput{
		CidrBlock: aws.String(awsvpc.Spec.CIDRBlock),
	}
	createVpcOutput, err := svc.CreateVpc(createVpcInput)
	if err != nil {
		awsvpc.Status.Status = "Error"
		awsvpc.Status.ErrorMessage = err.Error()
		return ctrl.Result{}, err
	}

	awsvpc.Status.VPCID = *createVpcOutput.Vpc.VpcId
	awsvpc.Status.Status = "Created"

	_, err = svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{createVpcOutput.Vpc.VpcId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(awsvpc.Spec.Name),
			},
		},
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create subnet
	createSubnetInput := &ec2.CreateSubnetInput{
		VpcId:     createVpcOutput.Vpc.VpcId,
		CidrBlock: aws.String(awsvpc.Spec.SubnetCIDR),
	}
	createSubnetOutput, err := svc.CreateSubnet(createSubnetInput)
	if err != nil {
		return ctrl.Result{}, err
	}

	awsvpc.Status.SubnetID = *createSubnetOutput.Subnet.SubnetId

	return ctrl.Result{}, nil
}

func (r *AWSVPCReconciler) updateVPC(svc *ec2.EC2, awsvpc *infrastructurev1.AWSVPC) (ctrl.Result, error) {
	// Check if CIDR block has changed
	describeVpcInput := &ec2.DescribeVpcsInput{
		VpcIds: []*string{aws.String(awsvpc.Status.VPCID)},
	}
	describeVpcOutput, err := svc.DescribeVpcs(describeVpcInput)
	if err != nil {
		return ctrl.Result{}, err
	}

	if *describeVpcOutput.Vpcs[0].CidrBlock != awsvpc.Spec.CIDRBlock {
		// CIDR block has changed, we need to create a new VPC
		// First, delete the old VPC and its resources
		if err := r.deleteVPCAndResources(svc, awsvpc); err != nil {
			return ctrl.Result{}, err
		}

		// Then create a new VPC
		return r.createVPC(svc, awsvpc)
	}

	// Update tags
	_, err = svc.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{aws.String(awsvpc.Status.VPCID)},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(awsvpc.Spec.Name),
			},
		},
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update subnet if CIDR has changed
	describeSubnetInput := &ec2.DescribeSubnetsInput{
		SubnetIds: []*string{aws.String(awsvpc.Status.SubnetID)},
	}
	describeSubnetOutput, err := svc.DescribeSubnets(describeSubnetInput)
	if err != nil {
		return ctrl.Result{}, err
	}

	if *describeSubnetOutput.Subnets[0].CidrBlock != awsvpc.Spec.SubnetCIDR {
		// Delete old subnet
		_, err = svc.DeleteSubnet(&ec2.DeleteSubnetInput{
			SubnetId: aws.String(awsvpc.Status.SubnetID),
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		// Create new subnet
		createSubnetInput := &ec2.CreateSubnetInput{
			VpcId:     aws.String(awsvpc.Status.VPCID),
			CidrBlock: aws.String(awsvpc.Spec.SubnetCIDR),
		}
		createSubnetOutput, err := svc.CreateSubnet(createSubnetInput)
		if err != nil {
			return ctrl.Result{}, err
		}

		awsvpc.Status.SubnetID = *createSubnetOutput.Subnet.SubnetId
	}

	awsvpc.Status.Status = "Updated"
	return ctrl.Result{}, nil
}

func (r *AWSVPCReconciler) deleteVPCAndResources(svc *ec2.EC2, awsvpc *infrastructurev1.AWSVPC) error {
	// Delete subnet
	_, err := svc.DeleteSubnet(&ec2.DeleteSubnetInput{
		SubnetId: aws.String(awsvpc.Status.SubnetID),
	})
	if err != nil {
		return err
	}

	// Delete VPC
	_, err = svc.DeleteVpc(&ec2.DeleteVpcInput{
		VpcId: aws.String(awsvpc.Status.VPCID),
	})
	if err != nil {
		return err
	}

	awsvpc.Status.VPCID = ""
	awsvpc.Status.SubnetID = ""
	awsvpc.Status.Status = "Deleted"

	return nil
}

func (r *AWSVPCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1.AWSVPC{}).
		Complete(r)
}
