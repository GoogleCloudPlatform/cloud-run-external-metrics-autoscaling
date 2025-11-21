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

package com.google.cloud.run.crema;

import static com.google.common.truth.Truth.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.google.cloud.run.crema.clients.KafkaAdminClientWrapper;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ExecutionException;
import org.apache.kafka.clients.admin.ConsumerGroupDescription;
import org.apache.kafka.clients.admin.ListOffsetsResult;
import org.apache.kafka.clients.admin.MemberAssignment;
import org.apache.kafka.clients.admin.MemberDescription;
import org.apache.kafka.clients.admin.OffsetSpec;
import org.apache.kafka.clients.admin.TopicDescription;
import org.apache.kafka.clients.consumer.OffsetAndMetadata;
import org.apache.kafka.common.ConsumerGroupState;
import org.apache.kafka.common.Node;
import org.apache.kafka.common.TopicPartition;
import org.apache.kafka.common.TopicPartitionInfo;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;
import org.mockito.ArgumentCaptor;

@RunWith(JUnit4.class)
public final class KafkaTest {

  private static final String TOPIC_NAME = "topic-name";
  private static final String CONSUMER_GROUP_ID = "consumer-group-id";

  private static final TopicDescription makeTopicDescription(
      String topicName, List<Integer> partitions) {
    Node node = new Node(/* id= */ 21, "host", /* port= */ 9999);

    ImmutableList.Builder<TopicPartitionInfo> topicPartitionInfos = ImmutableList.builder();
    for (int partition : partitions) {
      topicPartitionInfos.add(
          new TopicPartitionInfo(
              partition, node, /* replicas= */ ImmutableList.of(), /* isr= */ ImmutableList.of()));
    }

    return new TopicDescription(topicName, /* internal= */ false, topicPartitionInfos.build());
  }

  private static ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo>
      makeTopicOffsetsFromMap(Map<Integer, Long> partitionToOffset) {
    return makeTopicOffsetsFromMap(TOPIC_NAME, partitionToOffset);
  }

  private static ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo>
      makeTopicOffsetsFromMap(String topicName, Map<Integer, Long> partitionToOffset) {
    ImmutableMap.Builder<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> topicOffsets =
        ImmutableMap.builder();

    for (Map.Entry<Integer, Long> e : partitionToOffset.entrySet()) {
      TopicPartition topicPartition = new TopicPartition(topicName, e.getKey());
      ListOffsetsResult.ListOffsetsResultInfo listOffsetsResultInfo =
          new ListOffsetsResult.ListOffsetsResultInfo(
              e.getValue(), /* timestamp= */ 0, Optional.empty());
      topicOffsets.put(topicPartition, listOffsetsResultInfo);
    }
    return topicOffsets.buildOrThrow();
  }

  private static ImmutableMap<TopicPartition, OffsetAndMetadata> makeConsumerGroupOffsetsFromMap(
      Map<Integer, Long> partitionToOffset) {
    return makeConsumerGroupOffsetsFromMap(TOPIC_NAME, partitionToOffset);
  }

  private static ImmutableMap<TopicPartition, OffsetAndMetadata> makeConsumerGroupOffsetsFromMap(
      String topicName, Map<Integer, Long> partitionToOffset) {
    ImmutableMap.Builder<TopicPartition, OffsetAndMetadata> consumerGroupOffsets =
        ImmutableMap.builder();

    for (Map.Entry<Integer, Long> e : partitionToOffset.entrySet()) {
      TopicPartition topicPartition = new TopicPartition(topicName, e.getKey());
      OffsetAndMetadata offsetAndMetadata = new OffsetAndMetadata(e.getValue(), "metadata");
      consumerGroupOffsets.put(topicPartition, offsetAndMetadata);
    }
    return consumerGroupOffsets.buildOrThrow();
  }

  @Test
  public void doesTopicExist_topicExists_returnsTrue()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);
    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    when(kafkaAdminClientWrapper.listTopics()).thenReturn(ImmutableSet.of(TOPIC_NAME));
    assertThat(kafka.doesTopicExist(TOPIC_NAME)).isTrue();
    assertThat(kafka.doesTopicExist("non-existent-topic")).isFalse();
  }

  @Test
  public void doesTopicExist_topicDoesNotExist_returnsFalse()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);
    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    when(kafkaAdminClientWrapper.listTopics()).thenReturn(ImmutableSet.of(TOPIC_NAME));
    assertThat(kafka.doesTopicExist("non-existent-topic")).isFalse();
  }

  @Test
  public void getConsumerGroupSize_consumerGroupDoesNotExist_returnsZero()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);
    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    when(kafkaAdminClientWrapper.describeConsumerGroup(CONSUMER_GROUP_ID)).thenReturn(null);

    assertThat(kafka.getCurrentConsumerCount(CONSUMER_GROUP_ID)).isEqualTo(0);
  }

  @Test
  public void getConsumerGroupSize_withAssignedAndUnassignedConsumers_returnsTotalConsumerCount()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);
    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    Node coordinatorNode = new Node(/* id= */ 21, "coordinator_node", /* port= */ 9999);
    MemberDescription assignedConsumer =
        new MemberDescription(
            "memberId1",
            Optional.of("groupInstanceId1"),
            "clientId1",
            "host1",
            new MemberAssignment(ImmutableSet.of(new TopicPartition(TOPIC_NAME, 0))));
    MemberDescription unassignedConsumer =
        new MemberDescription(
            "memberId2",
            Optional.of("groupInstanceId2"),
            "clientId2",
            "host2",
            new MemberAssignment(ImmutableSet.of()));

    when(kafkaAdminClientWrapper.describeConsumerGroup(CONSUMER_GROUP_ID))
        .thenReturn(
            new ConsumerGroupDescription(
                CONSUMER_GROUP_ID,
                /* isSimpleConsumerGroup= */ true,
                /* members= */ ImmutableList.of(assignedConsumer, unassignedConsumer),
                "paritition_assignor",
                ConsumerGroupState.STABLE,
                coordinatorNode));

    assertThat(kafka.getCurrentConsumerCount(CONSUMER_GROUP_ID)).isEqualTo(2);
  }

  @Test
  public void getLagPerPartition_topicNotInDescribeTopicsResponse_returnsEmptyOptional()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);
    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    String topic = "topic-not-in-describe-topics-response";

    when(kafkaAdminClientWrapper.describeTopics(ImmutableSet.of(topic)))
        .thenReturn(ImmutableMap.of());
    assertThat(kafka.getLagPerPartition(Optional.of(topic), CONSUMER_GROUP_ID)).isEmpty();
  }

  @Test
  public void getLagPerPartition_noCommitedConsumerGroupOffsets_returnsSumOfTopicOffsets()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);

    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    TopicDescription topicDescription =
        makeTopicDescription(TOPIC_NAME, /* partitions= */ ImmutableList.of(0, 1));
    ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> topicOffsets =
        makeTopicOffsetsFromMap(ImmutableMap.of(0, 33L, 1, 200L));
    ImmutableMap<TopicPartition, OffsetAndMetadata> consumerGroupOffsets = ImmutableMap.of();

    when(kafkaAdminClientWrapper.describeTopics(any()))
        .thenReturn(ImmutableMap.of(TOPIC_NAME, topicDescription));
    when(kafkaAdminClientWrapper.listOffsets(any())).thenReturn(topicOffsets);
    when(kafkaAdminClientWrapper.listConsumerGroupOffsets(CONSUMER_GROUP_ID))
        .thenReturn(consumerGroupOffsets);

    assertThat(kafka.getLagPerPartition(Optional.of(TOPIC_NAME), CONSUMER_GROUP_ID).get())
        .containsExactly(
            new TopicPartition(TOPIC_NAME, 0), 33L, new TopicPartition(TOPIC_NAME, 1), 200L);
  }

  @Test
  public void getLagPerPartition_missingOneConsumerGroupOffsets_treatsMissingConsumerOffsetAsZero()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);

    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    TopicDescription topicDescription =
        makeTopicDescription(TOPIC_NAME, /* partitions= */ ImmutableList.of(0, 1));
    ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> topicOffsets =
        makeTopicOffsetsFromMap(ImmutableMap.of(0, 33L, 1, 200L));
    ImmutableMap<TopicPartition, OffsetAndMetadata> consumerGroupOffsets =
        makeConsumerGroupOffsetsFromMap(ImmutableMap.of(0, 5L));

    when(kafkaAdminClientWrapper.describeTopics(any()))
        .thenReturn(ImmutableMap.of(TOPIC_NAME, topicDescription));
    when(kafkaAdminClientWrapper.listOffsets(any())).thenReturn(topicOffsets);
    when(kafkaAdminClientWrapper.listConsumerGroupOffsets(CONSUMER_GROUP_ID))
        .thenReturn(consumerGroupOffsets);

    assertThat(kafka.getLagPerPartition(Optional.of(TOPIC_NAME), CONSUMER_GROUP_ID).get())
        .containsExactly(
            new TopicPartition(TOPIC_NAME, 0), 28L, new TopicPartition(TOPIC_NAME, 1), 200L);
  }

  @Test
  public void
      getLagPerPartition_consumerGroupOffsetGreaterThanTopicOffset_treatsConsumerOffsetAsTopicOffset()
          throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);

    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    TopicDescription topicDescription =
        makeTopicDescription(TOPIC_NAME, /* partitions= */ ImmutableList.of(0, 1));
    ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> topicOffsets =
        makeTopicOffsetsFromMap(ImmutableMap.of(0, 33L, 1, 200L));
    ImmutableMap<TopicPartition, OffsetAndMetadata> consumerGroupOffsets =
        makeConsumerGroupOffsetsFromMap(ImmutableMap.of(0, 5L, 1, 500L));

    when(kafkaAdminClientWrapper.describeTopics(any()))
        .thenReturn(ImmutableMap.of(TOPIC_NAME, topicDescription));
    when(kafkaAdminClientWrapper.listOffsets(any())).thenReturn(topicOffsets);
    when(kafkaAdminClientWrapper.listConsumerGroupOffsets(CONSUMER_GROUP_ID))
        .thenReturn(consumerGroupOffsets);

    assertThat(kafka.getLagPerPartition(Optional.of(TOPIC_NAME), CONSUMER_GROUP_ID).get())
        .containsExactly(
            new TopicPartition(TOPIC_NAME, 0), 28L, new TopicPartition(TOPIC_NAME, 1), 0L);
  }

  @Test
  public void
      getLagPerPartition_missingPartitionInTopicOffsets_ignoresCorrespondingConsumerGroupOffset()
          throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);

    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    TopicDescription topicDescription =
        makeTopicDescription(TOPIC_NAME, /* partitions= */ ImmutableList.of(0, 1));
    ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> topicOffsets =
        makeTopicOffsetsFromMap(ImmutableMap.of(0, 33L));
    ImmutableMap<TopicPartition, OffsetAndMetadata> consumerGroupOffsets =
        makeConsumerGroupOffsetsFromMap(ImmutableMap.of(0, 5L, 1, 100L));

    when(kafkaAdminClientWrapper.describeTopics(any()))
        .thenReturn(ImmutableMap.of(TOPIC_NAME, topicDescription));
    when(kafkaAdminClientWrapper.listOffsets(any())).thenReturn(topicOffsets);
    when(kafkaAdminClientWrapper.listConsumerGroupOffsets(CONSUMER_GROUP_ID))
        .thenReturn(consumerGroupOffsets);

    assertThat(kafka.getLagPerPartition(Optional.of(TOPIC_NAME), CONSUMER_GROUP_ID).get())
        .containsExactly(new TopicPartition(TOPIC_NAME, 0), 28L);
  }

  @Test
  public void getLagPerPartition_returnsDifferenceBetweenTopicAndConsumerGroupOffsets()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);

    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    TopicDescription topicDescription =
        makeTopicDescription(TOPIC_NAME, /* partitions= */ ImmutableList.of(0, 1));
    ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> topicOffsets =
        makeTopicOffsetsFromMap(ImmutableMap.of(0, 33L, 1, 200L));
    ImmutableMap<TopicPartition, OffsetAndMetadata> consumerGroupOffsets =
        makeConsumerGroupOffsetsFromMap(ImmutableMap.of(0, 5L, 1, 100L));

    when(kafkaAdminClientWrapper.describeTopics(any()))
        .thenReturn(ImmutableMap.of(TOPIC_NAME, topicDescription));
    when(kafkaAdminClientWrapper.listOffsets(any())).thenReturn(topicOffsets);
    when(kafkaAdminClientWrapper.listConsumerGroupOffsets(CONSUMER_GROUP_ID))
        .thenReturn(consumerGroupOffsets);

    assertThat(kafka.getLagPerPartition(Optional.of(TOPIC_NAME), CONSUMER_GROUP_ID).get())
        .containsExactly(
            new TopicPartition(TOPIC_NAME, 0), 28L, new TopicPartition(TOPIC_NAME, 1), 100L);

    @SuppressWarnings("unchecked")
    ArgumentCaptor<Map<TopicPartition, OffsetSpec>> listOffsetsArgumentCaptor =
        ArgumentCaptor.forClass(Map.class);
    verify(kafkaAdminClientWrapper).listOffsets(listOffsetsArgumentCaptor.capture());
    assertThat(listOffsetsArgumentCaptor.getValue().keySet())
        .containsExactly(new TopicPartition(TOPIC_NAME, 0), new TopicPartition(TOPIC_NAME, 1));
  }

  @Test
  public void getLagPerPartition_emptyTopicName_returnsLagForConsumerGroup()
      throws InterruptedException, ExecutionException {
    KafkaAdminClientWrapper kafkaAdminClientWrapper = mock(KafkaAdminClientWrapper.class);
    Kafka kafka = new Kafka(kafkaAdminClientWrapper);

    String topicName1 = "my-topic-1";
    String topicName2 = "my-topic-2";
    Map<TopicPartition, OffsetAndMetadata> consumerGroupOffsets = new HashMap<>();
    consumerGroupOffsets.putAll(
        makeConsumerGroupOffsetsFromMap(topicName1, ImmutableMap.of(0, 5L, 1, 100L)));
    consumerGroupOffsets.putAll(
        makeConsumerGroupOffsetsFromMap(topicName2, ImmutableMap.of(0, 20L, 1, 200L)));

    ImmutableMap<TopicPartition, ListOffsetsResult.ListOffsetsResultInfo> topicOffsets =
        makeTopicOffsetsFromMap(topicName1, ImmutableMap.of(0, 33L, 1, 200L));

    when(kafkaAdminClientWrapper.listConsumerGroupOffsets(CONSUMER_GROUP_ID))
        .thenReturn(ImmutableMap.copyOf(consumerGroupOffsets));
    when(kafkaAdminClientWrapper.listOffsets(any())).thenReturn(topicOffsets);

    assertThat(kafka.getLagPerPartition(Optional.empty(), CONSUMER_GROUP_ID).get())
        .containsExactly(
            new TopicPartition(topicName1, 0), 28L, new TopicPartition(topicName1, 1), 100L);

    @SuppressWarnings("unchecked")
    ArgumentCaptor<Map<TopicPartition, OffsetSpec>> listOffsetsArgumentCaptor =
        ArgumentCaptor.forClass(Map.class);
    verify(kafkaAdminClientWrapper).listOffsets(listOffsetsArgumentCaptor.capture());
    assertThat(listOffsetsArgumentCaptor.getValue().keySet())
        .containsExactly(
            new TopicPartition(topicName1, 0),
            new TopicPartition(topicName1, 1),
            new TopicPartition(topicName2, 0),
            new TopicPartition(topicName2, 1));
  }
}
