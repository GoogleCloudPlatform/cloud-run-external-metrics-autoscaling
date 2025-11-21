/*
 * Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package com.google.cloud.run.crema.clients;

import java.util.Collection;
import java.util.Collections;
import java.util.Map;
import java.util.Properties;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.ConsumerGroupDescription;
import org.apache.kafka.clients.admin.ListOffsetsResult;
import org.apache.kafka.clients.admin.OffsetSpec;
import org.apache.kafka.clients.admin.TopicDescription;
import org.apache.kafka.clients.consumer.OffsetAndMetadata;
import org.apache.kafka.common.TopicPartition;

/** Thin wrapper around Kafka Admin Client */
public class KafkaAdminClientWrapper {

  private final AdminClient adminClient;

  public KafkaAdminClientWrapper(Properties adminClientConfig) {
    this.adminClient = AdminClient.create(adminClientConfig);
  }

  public ConsumerGroupDescription describeConsumerGroup(String consumerGroupId)
      throws InterruptedException, ExecutionException {
    return adminClient
        .describeConsumerGroups(Collections.singletonList(consumerGroupId))
        .all()
        .get()
        .get(consumerGroupId);
  }

  public Set<String> listTopics() throws InterruptedException, ExecutionException {
    return adminClient.listTopics().names().get();
  }

  public Map<String, TopicDescription> describeTopics(Collection<String> topicNames)
      throws InterruptedException, ExecutionException {
    return adminClient.describeTopics(topicNames).allTopicNames().get();
  }

  public Map<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> listOffsets(
      Map<TopicPartition, OffsetSpec> topicPartitionOffsets)
      throws InterruptedException, ExecutionException {
    return adminClient.listOffsets(topicPartitionOffsets).all().get();
  }

  public Map<TopicPartition, OffsetAndMetadata> listConsumerGroupOffsets(String consumerGroupId)
      throws InterruptedException, ExecutionException {
    return adminClient
        .listConsumerGroupOffsets(consumerGroupId)
        .partitionsToOffsetAndMetadata()
        .get();
  }
}
